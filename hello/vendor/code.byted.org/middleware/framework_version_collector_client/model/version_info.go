package model

// be sure to in sync with code.byted.org/middleware/framework_version_collector/model
type CollectData struct {
	PSM           string      `json:"psm"`
	Cluster       string      `json:"cluster"`
	IDC           string      `json:"idc"`
	Env           string      `json:"env"`
	Name          string      `json:"name"`
	Version       string      `json:"version"`
	Extra         interface{} `json:"extra"`
	Protocol      string      `json:"protocol"`
	ProtocolExtra interface{} `json:"protocol_extra"`
}
