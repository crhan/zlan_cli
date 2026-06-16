package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/mqtt"
)

type mqttOptions struct {
	broker            string
	username          string
	password          string
	passwordFile      string
	passwordEnv       string
	clientID          string
	topic             string
	retain            bool
	field             string
	scale             float64
	offset            float64
	interval          time.Duration
	times             int
	haDiscovery       bool
	discoveryPrefix   string
	discoveryObjectID string
	name              string
	uniqueID          string
	unitOfMeasure     string
	deviceClass       string
	stateClass        string
	deviceID          string
	deviceName        string
	deviceModel       string
}

type mqttSample struct {
	Target    string
	Path      regDataPath
	Unit      byte
	Kind      string
	Address   uint16
	Values    []uint16
	Field     string
	Scale     float64
	Offset    float64
	Timestamp time.Time
	Topic     string
	Warnings  []string
}

func newMQTTCmd(g *globalFlags) *cobra.Command {
	regOpt := &regOptions{unit: 1, mode: "auto", kind: "holding"}
	opt := &mqttOptions{
		broker:          "localhost:1883",
		retain:          false,
		field:           "value",
		scale:           1,
		interval:        2 * time.Second,
		discoveryPrefix: "homeassistant",
		unitOfMeasure:   "",
		deviceClass:     "",
		stateClass:      "measurement",
		deviceModel:     "ZLAN RS485 gateway",
	}
	cmd := &cobra.Command{
		Use:   "mqtt",
		Short: "读取 ZLAN Modbus 寄存器并上送 MQTT",
		Long: `读取 ZLAN 数据通道上的 Modbus 寄存器,将数值封装成 JSON 发布到 MQTT broker。

这不是写入设备内部 JSON/MQTT 采集规则;它由 CLI 作为手动验证或外部调度的桥接进程,用于先把
RS485 数据稳定接入 Home Assistant MQTT。`,
	}
	cmd.PersistentFlags().UintVar(&regOpt.unit, "unit", 1, "Modbus 从站地址(1..247)")
	cmd.PersistentFlags().StringVar(&regOpt.mode, "mode", "auto", "数据通道协议:auto|modbus-tcp|rtu-over-tcp")
	cmd.PersistentFlags().StringVar(&regOpt.dataHost, "data-host", "", "覆盖数据通道主机/IP(默认取设备 local_ip)")
	cmd.PersistentFlags().IntVar(&regOpt.dataPort, "data-port", 0, "覆盖数据通道端口(默认取设备 local_port)")
	cmd.PersistentFlags().StringVar(&regOpt.kind, "kind", "holding", "读取类型:holding|input")

	cmd.PersistentFlags().StringVar(&opt.broker, "broker", opt.broker, "MQTT broker 地址(host[:port] 或 mqtt://host:port)")
	cmd.PersistentFlags().StringVar(&opt.username, "username", "", "MQTT 用户名")
	cmd.PersistentFlags().StringVar(&opt.password, "password", "", "MQTT 密码(优先用 --password-env 或 --password-file 避免进程参数暴露)")
	cmd.PersistentFlags().StringVar(&opt.passwordFile, "password-file", "", "从文件读取 MQTT 密码(去掉末尾换行)")
	cmd.PersistentFlags().StringVar(&opt.passwordEnv, "password-env", "", "从环境变量读取 MQTT 密码")
	cmd.PersistentFlags().StringVar(&opt.clientID, "client-id", "", "MQTT client id(默认按 target 生成)")
	cmd.PersistentFlags().StringVar(&opt.topic, "topic", "", "MQTT state topic(默认 zlan/<target>/state)")
	cmd.PersistentFlags().BoolVar(&opt.retain, "retain", false, "发布 retained state message")
	cmd.PersistentFlags().StringVar(&opt.field, "field", "value", "JSON payload 中主数值字段名")
	cmd.PersistentFlags().Float64Var(&opt.scale, "scale", 1, "主寄存器 raw 值乘数")
	cmd.PersistentFlags().Float64Var(&opt.offset, "offset", 0, "主寄存器 raw 值偏移量(先乘 scale 再加 offset)")

	cmd.PersistentFlags().BoolVar(&opt.haDiscovery, "ha-discovery", false, "同时发布 Home Assistant MQTT Discovery 配置(retain)")
	cmd.PersistentFlags().StringVar(&opt.discoveryPrefix, "discovery-prefix", opt.discoveryPrefix, "Home Assistant discovery prefix")
	cmd.PersistentFlags().StringVar(&opt.discoveryObjectID, "object-id", "", "HA discovery object id(默认按 unique-id 生成)")
	cmd.PersistentFlags().StringVar(&opt.name, "name", "", "HA 实体显示名称")
	cmd.PersistentFlags().StringVar(&opt.uniqueID, "unique-id", "", "HA 实体 unique_id")
	cmd.PersistentFlags().StringVar(&opt.unitOfMeasure, "unit-of-measurement", "", "HA 单位,如 \u00b0C")
	cmd.PersistentFlags().StringVar(&opt.deviceClass, "device-class", "", "HA device_class,如 temperature")
	cmd.PersistentFlags().StringVar(&opt.stateClass, "state-class", opt.stateClass, "HA state_class")
	cmd.PersistentFlags().StringVar(&opt.deviceID, "device-id", "", "HA device identifier")
	cmd.PersistentFlags().StringVar(&opt.deviceName, "device-name", "", "HA device name")
	cmd.PersistentFlags().StringVar(&opt.deviceModel, "device-model", opt.deviceModel, "HA device model")

	cmd.AddCommand(newMQTTPublishCmd(g, regOpt, opt), newMQTTBridgeCmd(g, regOpt, opt))
	return cmd
}

func newMQTTPublishCmd(g *globalFlags, regOpt *regOptions, opt *mqttOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "publish <target> <addr> [count]",
		Short: "读取一次寄存器并发布 MQTT",
		Example: `  zlan mqtt publish 192.168.15.47 0x0000 --unit 13 --broker localhost:1883 --topic zlan/water/rsds19y/state --field temperature --scale 0.1
  zlan mqtt publish 192.168.15.47 0x0000 --unit 13 --ha-discovery --name "RSDS19Y Temperature" --device-class temperature --unit-of-measurement °C`,
		Args: mqttReadArgs(g),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMQTTPublish(cmd, g, regOpt, opt, args)
		},
	}
}

func newMQTTBridgeCmd(g *globalFlags, regOpt *regOptions, opt *mqttOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge <target> <addr> [count]",
		Short: "按间隔读取寄存器并持续发布 MQTT",
		Example: `  zlan mqtt bridge 192.168.15.47 0x0000 --unit 13 --interval 2s --topic zlan/water/rsds19y/state --field temperature --scale 0.1
  zlan mqtt bridge 192.168.15.47 0x0000 --unit 13 --times 3 --json`,
		Args: mqttReadArgs(g),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMQTTBridge(cmd, g, regOpt, opt, args)
		},
	}
	cmd.Flags().DurationVar(&opt.interval, "interval", opt.interval, "轮询间隔")
	cmd.Flags().IntVar(&opt.times, "times", 0, "采样次数(0 表示一直运行到 Ctrl-C)")
	return cmd
}

func mqttReadArgs(g *globalFlags) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if g.serial != "" {
			return fmt.Errorf("mqtt 通过 ZLAN 网络数据通道读取寄存器,不能与 --serial 同用")
		}
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("需要:目标(IP 或 DevID/MAC)+ 起始寄存器地址 + 可选数量")
		}
		return nil
	}
}

func runMQTTPublish(cmd *cobra.Command, g *globalFlags, regOpt *regOptions, opt *mqttOptions, args []string) error {
	spec, err := parseMQTTReadSpec(regOpt, args)
	if err != nil {
		return exitErr(ExitUsage, err)
	}
	sample, err := readMQTTSample(cmd, g, regOpt, args[0], spec)
	if err != nil {
		return err
	}
	sample.Topic = mqttTopic(opt, sample.Target)
	return publishSample(cmd, g, opt, sample)
}

func runMQTTBridge(cmd *cobra.Command, g *globalFlags, regOpt *regOptions, opt *mqttOptions, args []string) error {
	if opt.interval <= 0 {
		return exitErr(ExitUsage, fmt.Errorf("--interval 必须大于 0"))
	}
	if opt.times < 0 {
		return exitErr(ExitUsage, fmt.Errorf("--times 不能为负数"))
	}
	spec, err := parseMQTTReadSpec(regOpt, args)
	if err != nil {
		return exitErr(ExitUsage, err)
	}

	var sent int
	for {
		sample, err := readMQTTSample(cmd, g, regOpt, args[0], spec)
		if err != nil {
			return err
		}
		sample.Topic = mqttTopic(opt, sample.Target)
		if err := publishSample(cmd, g, opt, sample); err != nil {
			return err
		}
		sent++
		if opt.times > 0 && sent >= opt.times {
			return nil
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-time.After(opt.interval):
		}
	}
}

type mqttReadSpec struct {
	addr  uint16
	count uint16
	fn    byte
	kind  string
	unit  byte
}

func parseMQTTReadSpec(regOpt *regOptions, args []string) (mqttReadSpec, error) {
	addr, err := parseUint16(args[1], "addr")
	if err != nil {
		return mqttReadSpec{}, err
	}
	count := uint16(1)
	if len(args) == 3 {
		count, err = parseUint16(args[2], "count")
		if err != nil {
			return mqttReadSpec{}, err
		}
	}
	if err := validateRegRange(addr, int(count), 125); err != nil {
		return mqttReadSpec{}, err
	}
	fn, kind, err := readKind(regOpt.kind)
	if err != nil {
		return mqttReadSpec{}, err
	}
	unit, err := parseUnit(regOpt.unit)
	if err != nil {
		return mqttReadSpec{}, err
	}
	return mqttReadSpec{addr: addr, count: count, fn: fn, kind: kind, unit: unit}, nil
}

func readMQTTSample(cmd *cobra.Command, g *globalFlags, regOpt *regOptions, target string, spec mqttReadSpec) (mqttSample, error) {
	var sample mqttSample
	err := withHost(cmd, g, target, func(ep *device.Endpoint) error {
		p, err := readRegParam(cmd, g, ep, target)
		if err != nil {
			return err
		}
		path, err := resolveRegDataPath(&p, regOpt)
		if err != nil {
			return err
		}
		emitRegWarnings(cmd, g, path.Warnings)
		sess, err := newRegSession(cmd.Context(), path, g.timeout)
		if err != nil {
			return err
		}
		defer sess.Close()
		values, _, err := sess.read(cmd.Context(), spec.unit, spec.fn, spec.addr, spec.count)
		if err != nil {
			return err
		}
		sample = mqttSample{
			Target:    target,
			Path:      path,
			Unit:      spec.unit,
			Kind:      spec.kind,
			Address:   spec.addr,
			Values:    values,
			Timestamp: time.Now(),
			Warnings:  path.Warnings,
		}
		return nil
	})
	return sample, err
}

func publishSample(cmd *cobra.Command, g *globalFlags, opt *mqttOptions, sample mqttSample) error {
	payload, rendered, err := mqttPayload(opt, sample)
	if err != nil {
		return exitErr(ExitUsage, err)
	}
	cfg, err := mqttConfig(opt, sample.Target)
	if err != nil {
		return exitErr(ExitUsage, err)
	}
	client, err := mqtt.Dial(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	if opt.haDiscovery {
		topic, body, err := mqttDiscovery(opt, sample)
		if err != nil {
			return exitErr(ExitUsage, err)
		}
		if err := client.Publish(topic, body, true); err != nil {
			return err
		}
	}
	if err := client.Publish(sample.Topic, payload, opt.retain); err != nil {
		return err
	}
	return renderMQTTPublish(cmd, g, opt, sample, rendered)
}

func mqttConfig(opt *mqttOptions, target string) (mqtt.Config, error) {
	addr, err := normalizeMQTTBroker(opt.broker)
	if err != nil {
		return mqtt.Config{}, err
	}
	password, err := mqttPassword(opt)
	if err != nil {
		return mqtt.Config{}, err
	}
	clientID := opt.clientID
	if clientID == "" {
		clientID = "zlan-" + safeID(target)
	}
	return mqtt.Config{
		Addr:     addr,
		ClientID: clientID,
		Username: opt.username,
		Password: password,
		Timeout:  opt.intervalOrTimeout(),
	}, nil
}

func (opt *mqttOptions) intervalOrTimeout() time.Duration {
	return 5 * time.Second
}

func mqttPassword(opt *mqttOptions) (string, error) {
	if opt.passwordEnv != "" {
		value, ok := os.LookupEnv(opt.passwordEnv)
		if !ok {
			return "", fmt.Errorf("环境变量 %s 未设置", opt.passwordEnv)
		}
		return value, nil
	}
	if opt.passwordFile != "" {
		data, err := os.ReadFile(opt.passwordFile)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return opt.password, nil
}

func normalizeMQTTBroker(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "mqtt://")
	if strings.HasPrefix(raw, "mqtts://") {
		return "", fmt.Errorf("当前仅支持明文 mqtt://,不支持 mqtts://")
	}
	if raw == "" {
		return "", fmt.Errorf("--broker 不能为空")
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw, nil
	}
	return net.JoinHostPort(raw, "1883"), nil
}

func mqttTopic(opt *mqttOptions, target string) string {
	if opt.topic != "" {
		return opt.topic
	}
	return "zlan/" + safeID(target) + "/state"
}

func mqttPayload(opt *mqttOptions, sample mqttSample) ([]byte, map[string]any, error) {
	field := strings.TrimSpace(opt.field)
	if field == "" {
		return nil, nil, fmt.Errorf("--field 不能为空")
	}
	if len(sample.Values) == 0 {
		return nil, nil, fmt.Errorf("没有寄存器值可发布")
	}
	primary := float64(sample.Values[0])*opt.scale + opt.offset
	rendered := map[string]any{
		"schema_version": jsonSchemaVersion,
		"target":         sample.Target,
		"data_address":   sample.Path.Address,
		"mode":           string(sample.Path.Mode),
		"work_mode":      sample.Path.WorkMode,
		"app_proto":      sample.Path.AppProto,
		"unit":           sample.Unit,
		"kind":           sample.Kind,
		"address":        sample.Address,
		"count":          len(sample.Values),
		"raw":            sample.Values[0],
		"values":         sample.Values,
		"scale":          opt.scale,
		"offset":         opt.offset,
		"ts":             sample.Timestamp.Format(time.RFC3339),
		field:            normalizeFloat(primary),
	}
	if len(sample.Warnings) > 0 {
		rendered["warnings"] = sample.Warnings
	}
	payload, err := json.Marshal(rendered)
	return payload, rendered, err
}

func normalizeFloat(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	if math.Trunc(v) == v {
		return int64(v)
	}
	return math.Round(v*1000000) / 1000000
}

func mqttDiscovery(opt *mqttOptions, sample mqttSample) (string, []byte, error) {
	stateTopic := sample.Topic
	uniqueID := opt.uniqueID
	if uniqueID == "" {
		uniqueID = "zlan_" + safeID(sample.Target) + "_u" + strconv.Itoa(int(sample.Unit)) + "_r" + fmt.Sprintf("%04x", sample.Address)
	}
	objectID := opt.discoveryObjectID
	if objectID == "" {
		objectID = safeID(uniqueID)
	}
	name := opt.name
	if name == "" {
		name = "ZLAN " + sample.Target + " register 0x" + fmt.Sprintf("%04X", sample.Address)
	}
	deviceID := opt.deviceID
	if deviceID == "" {
		deviceID = "zlan_" + safeID(sample.Target)
	}
	deviceName := opt.deviceName
	if deviceName == "" {
		deviceName = "ZLAN " + sample.Target
	}

	config := map[string]any{
		"name":           name,
		"unique_id":      uniqueID,
		"state_topic":    stateTopic,
		"value_template": "{{ value_json." + opt.field + " }}",
		"device": map[string]any{
			"identifiers":  []string{deviceID},
			"name":         deviceName,
			"manufacturer": "ZLAN",
			"model":        opt.deviceModel,
		},
	}
	if opt.unitOfMeasure != "" {
		config["unit_of_measurement"] = opt.unitOfMeasure
	}
	if opt.deviceClass != "" {
		config["device_class"] = opt.deviceClass
	}
	if opt.stateClass != "" {
		config["state_class"] = opt.stateClass
	}
	body, err := json.Marshal(config)
	if err != nil {
		return "", nil, err
	}
	prefix := strings.Trim(strings.TrimSpace(opt.discoveryPrefix), "/")
	if prefix == "" {
		return "", nil, fmt.Errorf("--discovery-prefix 不能为空")
	}
	return prefix + "/sensor/" + objectID + "/config", body, nil
}

func renderMQTTPublish(cmd *cobra.Command, g *globalFlags, opt *mqttOptions, sample mqttSample, payload map[string]any) error {
	if g.jsonOut {
		return writeJSONLine(cmd.OutOrStdout(), map[string]any{
			"schema_version": jsonSchemaVersion,
			"target":         sample.Target,
			"topic":          sample.Topic,
			"retain":         opt.retain,
			"payload":        payload,
		})
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "topic\t%s\n", sample.Topic)
	fmt.Fprintf(tw, "raw\t%d\n", sample.Values[0])
	fmt.Fprintf(tw, "%s\t%v\n", opt.field, payload[opt.field])
	fmt.Fprintf(tw, "ts\t%s\n", sample.Timestamp.Format(time.RFC3339))
	return tw.Flush()
}

func safeID(raw string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(raw) {
		ok := r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
		if !ok {
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
			continue
		}
		if r == '-' {
			r = '_'
		}
		b.WriteRune(r)
		lastUnderscore = r == '_'
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "zlan"
	}
	return out
}
