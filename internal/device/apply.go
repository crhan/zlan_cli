package device

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"zlan/internal/transport"
)

// ManifestItem 是批量清单中的一项:按 DevID(MAC)匹配,施加 Set 指定的字段。
// 按 DevID 而非 IP 匹配——IP 改完即变,不能作稳定主键。
type ManifestItem struct {
	DevID string
	Set   map[string]string
}

// ParseManifest 解析 YAML 批量清单。
//
//   - match: {devid: "aa:bb:cc:dd:ee:ff"}
//     set:   {local_ip: 10.0.0.11, net_mask: 255.255.255.0}
func ParseManifest(path string) ([]ManifestItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取清单失败: %w", err)
	}
	var raw []struct {
		Match struct {
			DevID string `yaml:"devid"`
		} `yaml:"match"`
		Set map[string]any `yaml:"set"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析清单 YAML 失败: %w", err)
	}
	items := make([]ManifestItem, 0, len(raw))
	for i, r := range raw {
		if r.Match.DevID == "" {
			return nil, fmt.Errorf("清单第 %d 项缺少 match.devid", i+1)
		}
		if len(r.Set) == 0 {
			return nil, fmt.Errorf("清单第 %d 项(%s)缺少 set", i+1, r.Match.DevID)
		}
		// yaml 会把无引号数字解析成 int,统一转成字符串交给字段注册表解析。
		set := make(map[string]string, len(r.Set))
		for k, v := range r.Set {
			set[k] = fmt.Sprint(v)
		}
		items = append(items, ManifestItem{DevID: r.Match.DevID, Set: set})
	}
	return items, nil
}

// ApplyOutcome 是批量中每台设备的结局。
type ApplyOutcome struct {
	DevID  string
	Found  bool // 是否在局域网发现该 DevID
	DryRun bool
	Result *SetResult
	Err    error
}

// Apply 批量应用清单:一次 discover 拿全部设备,按 DevID 匹配,逐台串行处理。
// dryRun 时只计算 diff 不写。某台失败不中断后续(逐项记录 Err)。
func Apply(ctx context.Context, items []ManifestItem, wait time.Duration, retries int, dryRun bool) ([]ApplyOutcome, error) {
	devs, err := transport.Discover(ctx, wait)
	if err != nil {
		return nil, err
	}
	byID := make(map[[6]byte]transport.Device, len(devs))
	for _, d := range devs {
		byID[d.Param.DevID()] = d
	}

	outcomes := make([]ApplyOutcome, 0, len(items))
	for _, it := range items {
		oc := ApplyOutcome{DevID: it.DevID, DryRun: dryRun}
		key, err := parseDevID(it.DevID)
		if err != nil {
			oc.Err = err
			outcomes = append(outcomes, oc)
			continue
		}
		dev, ok := byID[key]
		if !ok {
			outcomes = append(outcomes, oc) // Found=false
			continue
		}
		oc.Found = true
		snap := dev.Param
		if dryRun {
			oc.Result, oc.Err = planFrom(snap, it.Set)
			outcomes = append(outcomes, oc)
			continue
		}
		conn, derr := transport.Dial(dev.Addr, wait, retries)
		if derr != nil {
			oc.Err = derr
			outcomes = append(outcomes, oc)
			continue
		}
		oc.Result, oc.Err = Set(conn, &snap, it.Set, transport.WritePersist)
		_ = conn.Close()
		outcomes = append(outcomes, oc)
	}
	return outcomes, nil
}
