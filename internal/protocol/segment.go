package protocol

import (
	"fmt"
	"sort"
)

// Segment 是参数块内一段连续区间及其字节数据。
//
// 写操作统一建模为段集合:UDP 通道恒为单段全集(offset 0、167 字节),
// 串口通道按改动字段的最小覆盖区间切段——"整块 vs 部分"由此变成同一模型的
// 两种取值,而非两条代码路径。
type Segment struct {
	Offset int
	Data   []byte
}

// End 返回区间结束偏移(开区间)。
func (s Segment) End() int { return s.Offset + len(s.Data) }

// FullSegment 返回覆盖整个参数块的单段(UDP 通道用)。
func FullSegment(p *Param) []Segment {
	return []Segment{{Offset: 0, Data: append([]byte(nil), p[:]...)}}
}

// ChangedSegments 根据改动字段名,计算覆盖这些字段的最小连续段集合。
// 相邻/重叠的字段区间会合并成一段(连续区间一次写更高效);有空隙则分段,
// 空隙内未改字节不会被写入。位域子字段折算为其容器字节。
func ChangedSegments(p *Param, changed []string) ([]Segment, error) {
	type rng struct{ off, end int }
	ranges := make([]rng, 0, len(changed))
	for _, name := range changed {
		if bf, ok := bitFieldIndex[name]; ok {
			ranges = append(ranges, rng{bf.Byte, bf.Byte + 1})
			continue
		}
		f, ok := fieldIndex[name]
		if !ok {
			return nil, fmt.Errorf("未知字段:%s", name)
		}
		ranges = append(ranges, rng{f.Offset, f.Offset + f.Size})
	}
	if len(ranges) == 0 {
		return nil, nil
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].off < ranges[j].off })

	var segs []Segment
	cur := ranges[0]
	flush := func(r rng) {
		segs = append(segs, Segment{Offset: r.off, Data: append([]byte(nil), p[r.off:r.end]...)})
	}
	for _, r := range ranges[1:] {
		if r.off <= cur.end { // 相邻或重叠:合并
			if r.end > cur.end {
				cur.end = r.end
			}
			continue
		}
		flush(cur)
		cur = r
	}
	flush(cur)
	return segs, nil
}
