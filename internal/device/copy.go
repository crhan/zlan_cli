package device

import (
	"bytes"
	"fmt"
	"sort"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

// CopyOptions 控制从源设备复制哪些配置,以及复制后对目标做的显式覆盖。
type CopyOptions struct {
	Overrides       map[string]string
	Exclude         []string
	AllowSameDevice bool
}

// PlanCopy 以 target 为底板,把 source 的可写配置字段复制过来,但保留目标的
// DevID、状态、固件版本和能力位等只读身份字段。返回值只描述计划,不执行 I/O。
func PlanCopy(source, target protocol.Param, opt CopyOptions) (*SetResult, error) {
	if !opt.AllowSameDevice && source.DevID() == target.DevID() {
		return nil, fmt.Errorf("源设备和目标设备 DevID 相同,拒绝复制到自身")
	}

	ex, err := parseCopyExcludes(opt.Exclude)
	if err != nil {
		return nil, err
	}

	work := target.Clone()
	for _, f := range protocol.Fields() {
		if f.ReadOnly || ex.fields[f.Name] {
			continue
		}
		copy(work[f.Offset:f.Offset+f.Size], source[f.Offset:f.Offset+f.Size])
	}
	for _, bf := range ex.bits {
		mask := byte(1 << bf.Bit)
		work[bf.Byte] = (work[bf.Byte] &^ mask) | (target[bf.Byte] & mask)
	}
	for k, v := range opt.Overrides {
		if err := work.SetField(k, v); err != nil {
			return nil, err
		}
	}

	changed := changedCopyFields(target, work)
	return &SetResult{
		Before:       target,
		After:        work,
		Changed:      changed,
		NetworkField: touchesNetwork(changed),
	}, nil
}

// CopyConfig 读取目标当前参数,按 PlanCopy 生成目标参数,写入并尽力读回校验。
func CopyConfig(conn transport.Conn, snapshot *protocol.Param, source protocol.Param, opt CopyOptions) (*SetResult, error) {
	before, err := load(conn, snapshot)
	if err != nil {
		return nil, err
	}
	res, err := PlanCopy(source, before, opt)
	if err != nil {
		return nil, err
	}
	if len(res.Changed) == 0 {
		return res, nil
	}
	want := res.After
	if err := conn.WriteParam(want, res.Changed, transport.WritePersist); err != nil {
		return nil, err
	}
	if res.NetworkField {
		return res, nil
	}
	readback, err := conn.ReadParam()
	if err != nil {
		res.VerifyErr = err
		return res, nil
	}
	res.Verified = verify(&readback, &want, res.Changed)
	res.After = readback
	return res, nil
}

type copyExcludes struct {
	fields map[string]bool
	bits   []protocol.BitField
}

func parseCopyExcludes(names []string) (copyExcludes, error) {
	out := copyExcludes{fields: make(map[string]bool, len(names))}
	for _, name := range names {
		if name == "" {
			continue
		}
		if f, ok := protocol.FieldByName(name); ok {
			out.fields[f.Name] = true
			continue
		}
		if bf, ok := protocol.BitFieldByName(name); ok {
			out.bits = append(out.bits, bf)
			continue
		}
		return copyExcludes{}, fmt.Errorf("未知字段:%s", name)
	}
	return out, nil
}

func changedCopyFields(before, after protocol.Param) []string {
	var changed []string
	for _, f := range protocol.Fields() {
		if f.ReadOnly {
			continue
		}
		if !bytes.Equal(before[f.Offset:f.Offset+f.Size], after[f.Offset:f.Offset+f.Size]) {
			changed = append(changed, f.Name)
		}
	}
	sort.Strings(changed)
	return changed
}
