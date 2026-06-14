package device

import (
	"context"
	"fmt"
	"sort"
	"time"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

// Discover 广播发现局域网设备。
func Discover(ctx context.Context, wait time.Duration) ([]transport.Device, error) {
	return transport.Discover(ctx, wait)
}

// Status 读回设备连接状态(status@61 bit0)。
func Status(conn transport.Conn) (connected bool, p protocol.Param, err error) {
	p, err = conn.ReadParam()
	if err != nil {
		return false, p, err
	}
	return p.Connected(), p, nil
}

// networkFields 是改后会触发设备重启(且可能切换网段)的字段。
// set 这些字段后跳过即时读回校验,避免把"跨网段读不回"误报成失败。
var networkFields = map[string]bool{
	"local_ip": true, "net_mask": true, "gateway": true,
	"dhcp_en": true, "dns_server_ip": true,
}

// IsNetworkField 报告改该字段是否会触发设备重启(可能换网段),供上层预判确认文案。
func IsNetworkField(name string) bool { return networkFields[name] }

// SetResult 汇报一次 set 的结果。
type SetResult struct {
	Before       protocol.Param
	After        protocol.Param // 期望值;若做了读回校验则为读回值
	Changed      []string
	NetworkField bool  // 改了网络类字段(设备重启,可能换网段)
	Verified     bool  // 读回校验通过
	VerifyErr    error // 读回失败原因(设备重启时常为超时,非致命)
}

// Set 执行读-改-写:在快照(或现读)上应用 assignments,写回,非网络字段尝试读回校验。
func Set(conn transport.Conn, snapshot *protocol.Param, assignments map[string]string, mode transport.WriteMode) (*SetResult, error) {
	before, err := load(conn, snapshot)
	if err != nil {
		return nil, err
	}
	res, err := planFrom(before, assignments)
	if err != nil {
		return nil, err
	}
	want := res.After
	if err := conn.WriteParam(want, res.Changed, mode); err != nil {
		return nil, err
	}
	if res.NetworkField {
		return res, nil // 设备重启,跳过即时读回(避免跨网段误判失败)
	}
	readback, err := conn.ReadParam()
	if err != nil {
		res.VerifyErr = err // 多半是设备重启/无应答,非致命
		return res, nil
	}
	res.Verified = verify(&readback, &want, res.Changed)
	res.After = readback
	return res, nil
}

// planFrom 在 before 上应用 assignments,返回写计划(不发送),供 Set 与 dry-run 共用。
func planFrom(before protocol.Param, assignments map[string]string) (*SetResult, error) {
	if len(assignments) == 0 {
		return nil, fmt.Errorf("没有要修改的字段")
	}
	work := before.Clone()
	changed := make([]string, 0, len(assignments))
	for k, v := range assignments {
		if err := work.SetField(k, v); err != nil {
			return nil, err
		}
		changed = append(changed, k)
	}
	sort.Strings(changed) // 稳定展示顺序
	return &SetResult{Before: before, After: work, Changed: changed, NetworkField: touchesNetwork(changed)}, nil
}

func load(conn transport.Conn, snapshot *protocol.Param) (protocol.Param, error) {
	if snapshot != nil {
		return *snapshot, nil
	}
	return conn.ReadParam()
}

func touchesNetwork(changed []string) bool {
	for _, c := range changed {
		if networkFields[c] {
			return true
		}
	}
	return false
}

func verify(got, want *protocol.Param, fields []string) bool {
	for _, f := range fields {
		g, _ := got.GetField(f)
		w, _ := want.GetField(f)
		if g != w {
			return false
		}
	}
	return true
}
