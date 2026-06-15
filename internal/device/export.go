package device

import (
	"encoding/hex"
	"fmt"

	"gopkg.in/yaml.v3"

	"zlan/internal/protocol"
)

const ExportSchemaVersion = 1

// ExportedConfig 是 export/import 使用的离线配置文件格式。
// ParamHex 是权威数据源;Fields 只是给人查看的快照。
type ExportedConfig struct {
	SchemaVersion int               `json:"schema_version" yaml:"schema_version"`
	SourceDevID   string            `json:"source_devid" yaml:"source_devid"`
	ParamHex      string            `json:"param_hex" yaml:"param_hex"`
	Fields        map[string]string `json:"fields,omitempty" yaml:"fields,omitempty"`
}

func NewExportedConfig(p protocol.Param) ExportedConfig {
	fields := make(map[string]string, len(protocol.Fields()))
	for _, f := range protocol.Fields() {
		fields[f.Name], _ = p.GetField(f.Name)
	}
	devid, _ := p.GetField("devid")
	return ExportedConfig{
		SchemaVersion: ExportSchemaVersion,
		SourceDevID:   devid,
		ParamHex:      hex.EncodeToString(p[:]),
		Fields:        fields,
	}
}

func ParseExportedConfig(data []byte) (protocol.Param, ExportedConfig, error) {
	var cfg ExportedConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return protocol.Param{}, cfg, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if cfg.SchemaVersion != ExportSchemaVersion {
		return protocol.Param{}, cfg, fmt.Errorf("不支持的配置文件版本:%d", cfg.SchemaVersion)
	}
	raw, err := hex.DecodeString(cfg.ParamHex)
	if err != nil {
		return protocol.Param{}, cfg, fmt.Errorf("param_hex 不是合法 hex:%w", err)
	}
	p, err := protocol.Unmarshal(raw)
	if err != nil {
		return protocol.Param{}, cfg, fmt.Errorf("param_hex 无效: %w", err)
	}
	if cfg.SourceDevID == "" {
		cfg.SourceDevID, _ = p.GetField("devid")
	}
	return p, cfg, nil
}
