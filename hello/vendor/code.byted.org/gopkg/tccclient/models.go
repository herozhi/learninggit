package tccclient

const (
	KeyMetaFmt = "/tcc/v2/meta/%s"    // "/tcc/v2/meta/service"
	KeyDataFmt = "/tcc/v2/data/%s/%s" // "/tcc/v2/data/service/confspace"
)

type VersionCode struct {
	Confspace   string `json:"confspace"`
	VersionCode string `json:"version_code"`
}

type Meta struct {
	VersionCodes []*VersionCode `json:"version_codes"`
	ModifyTime   uint64         `json:"modify_time"`
}

type Data struct {
	Data        map[string]string `json:"data"`
	GrayData    *GrayData         `json:"gray_data,omitempty"`
	VersionCode string            `json:"version_code"`
	ModifyTime  uint64            `json:"modify_time"`

	NeedGray bool `json:"need_gray,omitempty"` // not stored in bconfig
}

type GrayData struct {
	Data       map[string]string `json:"data"`
	GrayCode   string            `json:"gray_code"`
	GrayIPList []string          `json:"gray_ip_list,omitempty"`
}

type GraySetting struct {
	Confspace string `json:"confspace"`
	GrayCode  string `json:"gray_code"`
}

type TCEGraySettings struct {
	GraySettings []*GraySetting `json:"gray_settings"`
}

func (m *Meta) GetVersionCode(confspace string) (string, bool) {
	for i := range m.VersionCodes {
		if m.VersionCodes[i].Confspace == confspace {
			return m.VersionCodes[i].VersionCode, true
		}
	}
	return "", false
}
