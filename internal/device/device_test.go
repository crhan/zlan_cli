package device

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

// fakeConn 模拟一台设备:WriteParam 把改动落到自己的 param,ReadParam 读回;
// readErrAfterWrite 模拟"写后设备重启、读不回"。
type fakeConn struct {
	param             protocol.Param
	writes            int
	lastMode          transport.WriteMode
	lastChanged       []string
	readErrAfterWrite bool
	written           bool
}

func (f *fakeConn) ReadParam() (protocol.Param, error) {
	if f.readErrAfterWrite && f.written {
		return protocol.Param{}, errors.New("设备重启,无应答")
	}
	return f.param, nil
}

func (f *fakeConn) WriteParam(p protocol.Param, changed []string, mode transport.WriteMode) error {
	f.param = p
	f.writes++
	f.lastMode = mode
	f.lastChanged = append([]string(nil), changed...)
	f.written = true
	return nil
}

func (f *fakeConn) Reboot() error { return nil }
func (f *fakeConn) Close() error  { return nil }

func TestSetNonNetworkVerified(t *testing.T) {
	f := &fakeConn{}
	res, err := Set(f, nil, map[string]string{"baud": "115200"}, transport.WritePersist)
	if err != nil {
		t.Fatal(err)
	}
	if res.NetworkField {
		t.Fatal("baud 不应判为网络字段")
	}
	if !res.Verified {
		t.Fatalf("应读回校验通过,VerifyErr=%v", res.VerifyErr)
	}
	if f.lastMode != transport.WritePersist {
		t.Fatal("写模式应为持久")
	}
	if got, _ := res.After.GetField("baud"); got != "115200" {
		t.Fatalf("after baud=%s", got)
	}
}

func TestSetNetworkSkipsVerify(t *testing.T) {
	f := &fakeConn{}
	res, err := Set(f, nil, map[string]string{"local_ip": "10.0.0.9"}, transport.WritePersist)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NetworkField {
		t.Fatal("local_ip 应判为网络字段")
	}
	if res.Verified {
		t.Fatal("网络字段应跳过即时读回校验")
	}
}

func TestSetVerifyFailOnReboot(t *testing.T) {
	f := &fakeConn{readErrAfterWrite: true}
	res, err := Set(f, nil, map[string]string{"baud": "9600"}, transport.WritePersist)
	if err != nil {
		t.Fatalf("读回失败不应算 Set 失败: %v", err)
	}
	if res.Verified {
		t.Fatal("读回失败不应标记已校验")
	}
	if res.VerifyErr == nil {
		t.Fatal("应记录读回失败原因")
	}
}

func TestSetRejectsInvalidAndReadOnly(t *testing.T) {
	f := &fakeConn{}
	for _, kv := range []map[string]string{
		{"bogus": "1"},
		{"ver": "1"},
		{"work_mode": "nope"},
	} {
		if _, err := Set(f, nil, kv, transport.WritePersist); err == nil {
			t.Errorf("%v 应被拒绝", kv)
		}
	}
	if f.writes != 0 {
		t.Fatal("非法 set 不应写入设备")
	}
}

func TestPlanCopyPreservesTargetIdentityAndCopiesWritableBytes(t *testing.T) {
	var source, target protocol.Param
	copy(source[31:37], []byte{0x5a, 0x4c, 0x6f, 0x73, 0xcc, 0xd6})
	copy(target[31:37], []byte{0x5a, 0x4c, 0x6f, 0x73, 0xcc, 0xd7})
	source[103] = 0x44 // ver, read-only
	target[103] = 0x55
	if err := source.SetField("local_ip", "10.0.0.9"); err != nil {
		t.Fatal(err)
	}
	if err := target.SetField("local_ip", "10.0.0.10"); err != nil {
		t.Fatal(err)
	}
	if err := source.SetField("baud", "115200"); err != nil {
		t.Fatal(err)
	}
	source[115] = 0xaa // user_param, opaque but writable through copy

	res, err := PlanCopy(source, target, CopyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.After.DevID() != target.DevID() {
		t.Fatal("copy 不应覆盖目标 DevID")
	}
	if res.After[103] != target[103] {
		t.Fatal("copy 不应覆盖目标 ver")
	}
	if got, _ := res.After.GetField("local_ip"); got != "10.0.0.9" {
		t.Fatalf("local_ip=%s", got)
	}
	if got, _ := res.After.GetField("baud"); got != "115200" {
		t.Fatalf("baud=%s", got)
	}
	if res.After[115] != 0xaa {
		t.Fatalf("user_param 未复制:0x%02x", res.After[115])
	}
	if !res.NetworkField {
		t.Fatal("复制 local_ip 应判为网络字段")
	}
}

func TestPlanCopyExcludeAndOverride(t *testing.T) {
	var source, target protocol.Param
	copy(source[31:37], []byte{0, 1, 2, 3, 4, 5})
	copy(target[31:37], []byte{0, 1, 2, 3, 4, 6})
	if err := source.SetField("local_ip", "10.0.0.9"); err != nil {
		t.Fatal(err)
	}
	if err := target.SetField("local_ip", "10.0.0.10"); err != nil {
		t.Fatal(err)
	}
	if err := source.SetField("baud", "115200"); err != nil {
		t.Fatal(err)
	}
	if err := source.SetField("func_en", "0xff"); err != nil {
		t.Fatal(err)
	}

	res, err := PlanCopy(source, target, CopyOptions{
		Exclude:   []string{"local_ip", "func_en.need_password"},
		Overrides: map[string]string{"baud": "57600"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := res.After.GetField("local_ip"); got != "10.0.0.10" {
		t.Fatalf("local_ip 应保留目标值,got %s", got)
	}
	if got, _ := res.After.GetField("baud"); got != "57600" {
		t.Fatalf("baud override=%s", got)
	}
	if got, _ := res.After.GetField("func_en"); got != "0xfb" {
		t.Fatalf("func_en bit exclude got %s", got)
	}
}

func TestPlanCopyRejectsSameDevice(t *testing.T) {
	var source, target protocol.Param
	copy(source[31:37], []byte{0, 1, 2, 3, 4, 5})
	copy(target[31:37], []byte{0, 1, 2, 3, 4, 5})
	if _, err := PlanCopy(source, target, CopyOptions{}); err == nil {
		t.Fatal("复制到相同 DevID 应报错")
	}
	if _, err := PlanCopy(source, target, CopyOptions{AllowSameDevice: true}); err != nil {
		t.Fatalf("import 恢复同一 DevID 应允许:%v", err)
	}
}

func TestCopyConfigNoWriteWhenUnchanged(t *testing.T) {
	var source, target protocol.Param
	copy(source[31:37], []byte{0, 1, 2, 3, 4, 5})
	copy(target[31:37], []byte{0, 1, 2, 3, 4, 6})
	f := &fakeConn{param: target}
	res, err := CopyConfig(f, nil, source, CopyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changed) != 0 {
		t.Fatalf("changed=%v", res.Changed)
	}
	if f.writes != 0 {
		t.Fatalf("无变化不应写入,writes=%d", f.writes)
	}
}

func TestExportedConfigRoundTrip(t *testing.T) {
	var p protocol.Param
	copy(p[31:37], []byte{0x5a, 0x4c, 0x6f, 0x73, 0xcc, 0xd6})
	if err := p.SetField("local_ip", "10.0.0.9"); err != nil {
		t.Fatal(err)
	}
	p[115] = 0xaa

	cfg := NewExportedConfig(p)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, parsed, err := ParseExportedConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatal("export/import 后参数块不一致")
	}
	if parsed.SourceDevID != "5a:4c:6f:73:cc:d6" {
		t.Fatalf("source_devid=%s", parsed.SourceDevID)
	}
	if parsed.Fields["local_ip"] != "10.0.0.9" {
		t.Fatalf("fields.local_ip=%s", parsed.Fields["local_ip"])
	}
}

func TestParseExportedConfigValidation(t *testing.T) {
	if _, _, err := ParseExportedConfig([]byte("schema_version: 99\nparam_hex: \"00\"\n")); err == nil {
		t.Fatal("未知 schema_version 应报错")
	}
	if _, _, err := ParseExportedConfig([]byte("schema_version: 1\nparam_hex: nope\n")); err == nil {
		t.Fatal("非法 hex 应报错")
	}
	if _, _, err := ParseExportedConfig([]byte("schema_version: 1\nparam_hex: \"00\"\n")); err == nil {
		t.Fatal("错误长度应报错")
	}
}

func TestPlanNetworkDetection(t *testing.T) {
	var before protocol.Param
	res, err := planFrom(before, map[string]string{"local_ip": "10.0.0.1", "baud": "9600"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.NetworkField {
		t.Fatal("含 local_ip 应判网络字段")
	}
	if len(res.Changed) != 2 {
		t.Fatalf("应有 2 个改动字段,got %d", len(res.Changed))
	}
}

func TestParseManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.yaml")
	content := `- match: {devid: "5a:4c:6f:73:cc:d6"}
  set: {local_ip: 10.0.0.11, net_mask: 255.255.255.0, baud: 115200}
- match: {devid: "5a:4c:6f:73:cc:d7"}
  set: {local_ip: 10.0.0.12}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("应解析 2 项,got %d", len(items))
	}
	if items[0].DevID != "5a:4c:6f:73:cc:d6" {
		t.Fatalf("devid=%s", items[0].DevID)
	}
	if items[0].Set["local_ip"] != "10.0.0.11" {
		t.Fatalf("local_ip=%s", items[0].Set["local_ip"])
	}
	if items[0].Set["baud"] != "115200" { // yaml int 应转成字符串
		t.Fatalf("baud=%q", items[0].Set["baud"])
	}
}

func TestParseManifestValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("- set: {local_ip: 1.2.3.4}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(path); err == nil {
		t.Fatal("缺 match.devid 应报错")
	}
}

func TestParseDevID(t *testing.T) {
	if _, err := parseDevID("5a:4c:6f:73:cc:d6"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseDevID("not-a-mac"); err == nil {
		t.Fatal("非法 MAC 应报错")
	}
}
