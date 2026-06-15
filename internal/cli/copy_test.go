package cli

import "testing"

func TestParseCopyArgs(t *testing.T) {
	source, target, overrides, err := parseCopyArgs([]string{
		"192.168.1.200",
		"192.168.1.201",
		"local_ip=192.168.1.202",
		"func_en.need_password=1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if source != "192.168.1.200" || target != "192.168.1.201" {
		t.Fatalf("source=%q target=%q", source, target)
	}
	if overrides["local_ip"] != "192.168.1.202" {
		t.Fatalf("local_ip override=%q", overrides["local_ip"])
	}
	if overrides["func_en.need_password"] != "1" {
		t.Fatalf("bit override=%q", overrides["func_en.need_password"])
	}
}

func TestParseCopyArgsValidation(t *testing.T) {
	if _, _, _, err := parseCopyArgs([]string{"192.168.1.200"}); err == nil {
		t.Fatal("缺 target 应报错")
	}
	if _, _, _, err := parseCopyArgs([]string{"a", "b", "not-kv"}); err == nil {
		t.Fatal("非法 override 格式应报错")
	}
	if _, _, _, err := parseCopyArgs([]string{"a", "b", "bogus=1"}); err == nil {
		t.Fatal("未知字段应报错")
	}
}
