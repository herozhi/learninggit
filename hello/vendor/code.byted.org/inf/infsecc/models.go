package infsecc

type Identity struct {
	Version         int64             `json:"version"`
	Authority       string            `json:"authority"`
	AuthorityChain  []string          `json:"authorityChain,omitempty"`
	PrimaryAuthType string            `json:"primaryAuthType"`
	PSM             string            `json:"psm"`
	User            string            `json:"user"`
	ExpireTime      int64             `json:"expireTime"`
	Extension       map[string]string `json:"extension,omitempty"`
}
