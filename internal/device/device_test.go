package device

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
