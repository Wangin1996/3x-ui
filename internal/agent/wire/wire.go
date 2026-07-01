package wire

import "github.com/mhsanaei/3x-ui/v3/internal/xray"

const HeaderNodeGuid = "X-Node-Guid"

type ConfigResponse struct {
	Unchanged bool         `json:"unchanged" example:"false"`
	Sha256    string       `json:"sha256,omitempty" example:"3b1f...c9"`
	Config    *xray.Config `json:"config,omitempty"`
}

type SysMetrics struct {
	CpuPct      float64 `json:"cpuPct" example:"23.5"`
	MemPct      float64 `json:"memPct" example:"45.1"`
	UptimeSecs  uint64  `json:"uptimeSecs" example:"86400"`
	NetUp       uint64  `json:"netUp" example:"1048576"`
	NetDown     uint64  `json:"netDown" example:"2097152"`
	XrayState   string  `json:"xrayState" example:"running"`
	XrayError   string  `json:"xrayError" example:""`
	XrayVersion string  `json:"xrayVersion" example:"25.10.31"`
}

type Report struct {
	InboundTraffics []*xray.Traffic       `json:"inboundTraffics"`
	ClientTraffics  []*xray.ClientTraffic `json:"clientTraffics"`
	OnlineEmails    []string              `json:"onlineEmails"`
	ClientIps       map[string][]string   `json:"clientIps"`
	Sys             SysMetrics            `json:"sys"`
}

type ReportResponse struct {
	Ok          bool                  `json:"ok" example:"true"`
	Globals     []*xray.ClientTraffic `json:"globals,omitempty"`
	ConfigDirty bool                  `json:"configDirty" example:"false"`
}
