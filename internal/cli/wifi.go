package cli

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
	"zlan/internal/transport"
)

func newWiFiCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "读取或修改 WiFi 参数",
		Long: `读取或修改 Rev.4 UDP 管理协议 user_param@115 中的 WiFi TLV 参数。

WiFi 写入会保存参数并使设备重启。密码默认脱敏显示。`,
	}
	cmd.AddCommand(newWiFiGetCmd(g), newWiFiSetCmd(g))
	return cmd
}

func newWiFiGetCmd(g *globalFlags) *cobra.Command {
	var showKey bool
	cmd := &cobra.Command{
		Use:   "get [target]",
		Short: "读取 WiFi 参数",
		Args:  targetArgs(g),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				p, err := readParam(ep)
				if err != nil {
					return err
				}
				cfg, err := wifiConfigFromParam(&p)
				if err != nil {
					return err
				}
				return renderWiFiConfig(cmd.OutOrStdout(), cfg, showKey, g.jsonOut)
			})
		},
	}
	cmd.Flags().BoolVar(&showKey, "show-key", false, "显示 WiFi 密码(默认脱敏)")
	return cmd
}

func newWiFiSetCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [target] <field=value>...",
		Short: "修改 WiFi 参数(保存并重启)",
		Long: `修改 WiFi 参数。字段:
  ssid=<name>
  key=<password> 或 password=<password>
  mode=ap|sta
  crypt=none|wep64|tkip|aes|wep128|auto
  channel=1..11
  dhcp_server=enabled|disabled
  bridge=enabled|disabled

示例:
  zlan wifi set 192.168.1.200 ssid=TP-LINK key=12345678 mode=sta crypt=auto channel=6 dhcp_server=disabled bridge=disabled -y`,
		RunE: func(cmd *cobra.Command, args []string) error {
			host, kvArgs, err := parseWiFiWriteArgs(g, args)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			label := targetLabel(g, host)
			ok, err := confirm(cmd, g, "将修改 %s 的 WiFi 参数并保存重启设备", label)
			if err != nil {
				return err
			}
			if !ok {
				cmd.PrintErrln("已取消")
				return nil
			}

			return withHost(cmd, g, host, func(ep *device.Endpoint) error {
				res, err := setWiFi(ep.Conn, ep.Snapshot, kvArgs)
				if err != nil {
					return err
				}
				return renderWiFiSetResult(cmd, g, res)
			})
		},
	}
	return cmd
}

type wifiSetResult struct {
	Before         protocol.WiFiConfig
	After          protocol.WiFiConfig
	Changed        []string
	Wrote          bool
	RebootExpected bool
	Verified       bool
	VerifyErr      error
}

func parseWiFiWriteArgs(g *globalFlags, args []string) (host string, kvs map[string]string, err error) {
	kvArgs := args
	if g.serial == "" {
		if len(args) < 1 {
			return "", nil, fmt.Errorf("需要设备目标(IP 或 DevID/MAC)")
		}
		host = args[0]
		kvArgs = args[1:]
	}
	if len(kvArgs) == 0 {
		return "", nil, fmt.Errorf("至少给一个 WiFi field=value")
	}
	kvs = make(map[string]string, len(kvArgs))
	for _, a := range kvArgs {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return "", nil, fmt.Errorf("参数格式应为 field=value:%q", a)
		}
		k = strings.ToLower(strings.TrimSpace(k))
		switch k {
		case "ssid", "key", "password", "mode", "crypt", "encryption", "channel", "dhcp_server", "bridge":
			if k == "password" {
				k = "key"
			}
			if k == "encryption" {
				k = "crypt"
			}
			kvs[k] = v
		default:
			return "", nil, fmt.Errorf("未知 WiFi 字段:%s", k)
		}
	}
	return host, kvs, nil
}

func setWiFi(conn transport.Conn, snapshot *protocol.Param, kvs map[string]string) (*wifiSetResult, error) {
	beforeParam, err := loadWiFiParam(conn, snapshot)
	if err != nil {
		return nil, err
	}
	records, err := beforeParam.UserTLVs()
	if err != nil {
		return nil, err
	}
	beforeCfg, err := protocol.WiFiConfigFromTLVs(records)
	if err != nil {
		return nil, err
	}
	afterCfg, changed, err := applyWiFiAssignments(beforeCfg, kvs)
	if err != nil {
		return nil, err
	}
	nextRecords, err := protocol.ReplaceWiFiConfig(records, afterCfg)
	if err != nil {
		return nil, err
	}
	afterParam := beforeParam.Clone()
	if err := afterParam.SetUserTLVs(nextRecords); err != nil {
		return nil, err
	}
	res := &wifiSetResult{Before: beforeCfg, After: afterCfg, Changed: changed}
	if bytes.Equal(beforeParam[protocol.UserParamOffset:protocol.UserParamOffset+protocol.UserParamLen],
		afterParam[protocol.UserParamOffset:protocol.UserParamOffset+protocol.UserParamLen]) {
		return res, nil
	}
	if err := conn.WriteParam(afterParam, []string{"user_param"}, transport.WritePersist); err != nil {
		return nil, err
	}
	res.Wrote = true
	res.RebootExpected = transport.PersistentWriteReboots(conn)
	if res.RebootExpected {
		return res, nil
	}
	readback, err := conn.ReadParam()
	if err != nil {
		res.VerifyErr = err
		return res, nil
	}
	res.Verified = bytes.Equal(readback[protocol.UserParamOffset:protocol.UserParamOffset+protocol.UserParamLen],
		afterParam[protocol.UserParamOffset:protocol.UserParamOffset+protocol.UserParamLen])
	return res, nil
}

func loadWiFiParam(conn transport.Conn, snapshot *protocol.Param) (protocol.Param, error) {
	if snapshot != nil {
		return *snapshot, nil
	}
	return conn.ReadParam()
}

func wifiConfigFromParam(p *protocol.Param) (protocol.WiFiConfig, error) {
	records, err := p.UserTLVs()
	if err != nil {
		return protocol.WiFiConfig{}, err
	}
	return protocol.WiFiConfigFromTLVs(records)
}

func applyWiFiAssignments(base protocol.WiFiConfig, kvs map[string]string) (protocol.WiFiConfig, []string, error) {
	cfg := base
	changed := make([]string, 0, len(kvs))
	needsChannelByte := false
	modeTouched := false
	cryptTouched := false

	for k, v := range kvs {
		switch k {
		case "ssid":
			cfg.SSID = v
			cfg.HasSSID = true
			changed = append(changed, "ssid")
		case "key", "password":
			cfg.Key = v
			cfg.HasKey = true
			changed = append(changed, "key")
		case "channel":
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 1 || n > 11 {
				return cfg, nil, fmt.Errorf("channel 必须是 1..11")
			}
			cfg.Channel = n
			cfg.HasChannel = true
			changed = append(changed, "channel")
		case "dhcp_server":
			enabled, err := parseEnabled(v)
			if err != nil {
				return cfg, nil, fmt.Errorf("dhcp_server: %w", err)
			}
			cfg.DHCPServerDisabled = !enabled
			needsChannelByte = true
			changed = append(changed, "dhcp_server")
		case "bridge":
			enabled, err := parseEnabled(v)
			if err != nil {
				return cfg, nil, fmt.Errorf("bridge: %w", err)
			}
			cfg.EthWiFiBridge = enabled
			needsChannelByte = true
			changed = append(changed, "bridge")
		case "mode":
			mode := strings.ToLower(strings.TrimSpace(v))
			if mode != "ap" && mode != "sta" && mode != "station" {
				return cfg, nil, fmt.Errorf("mode 可选:ap | sta")
			}
			if mode == "station" {
				mode = "sta"
			}
			cfg.Mode = mode
			cfg.HasModeCrypt = true
			modeTouched = true
			changed = append(changed, "mode")
		case "crypt", "encryption":
			crypt := strings.ToLower(strings.TrimSpace(v))
			if _, err := protocol.ReplaceWiFiConfig(nil, protocol.WiFiConfig{Mode: "sta", Crypt: crypt, HasModeCrypt: true}); err != nil {
				return cfg, nil, err
			}
			cfg.Crypt = crypt
			cfg.HasModeCrypt = true
			cryptTouched = true
			changed = append(changed, "crypt")
		}
	}
	if needsChannelByte && !cfg.HasChannel {
		return cfg, nil, fmt.Errorf("dhcp_server/bridge 与 channel 共用同一个 TLV;设备原值无 channel,请同时设置 channel=1..11")
	}
	if cfg.HasModeCrypt && (cfg.Mode == "" || cfg.Crypt == "") {
		missing := "mode 和 crypt"
		if modeTouched {
			missing = "crypt"
		}
		if cryptTouched {
			missing = "mode"
		}
		return cfg, nil, fmt.Errorf("mode/crypt 共用同一个 TLV;设备原值不完整,请同时设置 %s", missing)
	}
	return cfg, uniqueWiFiChanged(changed), nil
}

func parseEnabled(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes", "enable", "enabled":
		return true, nil
	case "0", "false", "off", "no", "disable", "disabled":
		return false, nil
	}
	return false, fmt.Errorf("取值需 enabled/disabled 或 on/off")
}

func uniqueWiFiChanged(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, name := range in {
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func renderWiFiConfig(w interface {
	Write([]byte) (int, error)
}, cfg protocol.WiFiConfig, showKey bool, asJSON bool) error {
	if asJSON {
		m := wifiJSON(cfg, showKey)
		m["schema_version"] = jsonSchemaVersion
		return writeJSON(w, m)
	}
	rows := [][]string{
		{"ssid", wifiValue(cfg, "ssid", showKey)},
		{"mode", wifiValue(cfg, "mode", showKey)},
		{"crypt", wifiValue(cfg, "crypt", showKey)},
		{"channel", wifiValue(cfg, "channel", showKey)},
		{"dhcp_server", wifiValue(cfg, "dhcp_server", showKey)},
		{"bridge", wifiValue(cfg, "bridge", showKey)},
		{"key", wifiValue(cfg, "key", showKey)},
	}
	return writeAlignedTable(w, []tableColumn{{header: "FIELD"}, {header: "VALUE"}}, rows)
}

func renderWiFiSetResult(cmd *cobra.Command, g *globalFlags, res *wifiSetResult) error {
	if g.jsonOut {
		changes := make(map[string]map[string]string, len(res.Changed))
		for _, f := range res.Changed {
			changes[f] = map[string]string{
				"before": wifiValue(res.Before, f, false),
				"after":  wifiValue(res.After, f, false),
			}
		}
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"schema_version":  jsonSchemaVersion,
			"changed":         changes,
			"wrote":           res.Wrote,
			"reboot_expected": res.RebootExpected,
			"verified":        res.Verified,
		})
	}
	out := cmd.OutOrStdout()
	for _, f := range res.Changed {
		fmt.Fprintf(out, "%s: %s -> %s\n", f, wifiValue(res.Before, f, false), wifiValue(res.After, f, false))
	}
	switch {
	case !res.Wrote:
		cmd.PrintErrln("无变化")
	case res.RebootExpected:
		cmd.PrintErrln(yellow("已写入,设备应保存参数并重启;稍后用 wifi get/info 确认。"))
	case res.Verified:
		cmd.PrintErrln(green("已生效(读回校验通过)。"))
	default:
		cmd.PrintErrf(yellow("已写入;读回未确认(%v),设备可能正在重启,稍后用 wifi get/info 确认。\n"), res.VerifyErr)
	}
	return nil
}

func wifiJSON(cfg protocol.WiFiConfig, showKey bool) map[string]any {
	out := map[string]any{
		"ssid":                 nil,
		"mode":                 nil,
		"crypt":                nil,
		"channel":              nil,
		"dhcp_server_enabled":  nil,
		"eth_wifi_bridge":      nil,
		"key":                  nil,
		"key_set":              cfg.HasKey,
		"has_channel_tlv":      cfg.HasChannel,
		"has_mode_crypt_tlv":   cfg.HasModeCrypt,
		"has_ssid_tlv":         cfg.HasSSID,
		"has_password_key_tlv": cfg.HasKey,
	}
	if cfg.HasSSID {
		out["ssid"] = cfg.SSID
	}
	if cfg.HasModeCrypt {
		out["mode"] = cfg.Mode
		out["crypt"] = cfg.Crypt
	}
	if cfg.HasChannel {
		out["channel"] = cfg.Channel
		out["dhcp_server_enabled"] = !cfg.DHCPServerDisabled
		out["eth_wifi_bridge"] = cfg.EthWiFiBridge
	}
	if cfg.HasKey && showKey {
		out["key"] = cfg.Key
	}
	return out
}

func wifiValue(cfg protocol.WiFiConfig, field string, showKey bool) string {
	switch field {
	case "ssid":
		if cfg.HasSSID {
			return cfg.SSID
		}
	case "key", "password":
		if cfg.HasKey {
			if showKey {
				return cfg.Key
			}
			return "<set>"
		}
	case "mode":
		if cfg.HasModeCrypt {
			return cfg.Mode
		}
	case "crypt", "encryption":
		if cfg.HasModeCrypt {
			return cfg.Crypt
		}
	case "channel":
		if cfg.HasChannel {
			return strconv.Itoa(cfg.Channel)
		}
	case "dhcp_server":
		if cfg.HasChannel {
			if cfg.DHCPServerDisabled {
				return "disabled"
			}
			return "enabled"
		}
	case "bridge":
		if cfg.HasChannel {
			if cfg.EthWiFiBridge {
				return "enabled"
			}
			return "disabled"
		}
	}
	return "-"
}
