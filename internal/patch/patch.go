package patch

type Patch struct {
	Version string   `json:"version"`
	Path    string   `json:"path,omitempty"`
	Unsaved bool     `json:"unsaved,omitempty"`
	Zoom    float64  `json:"zoom,omitempty"`
	Modules []Module `json:"modules"`
	Cables  []Cable  `json:"cables,omitempty"`
	Wires   []Cable  `json:"wires,omitempty"`
}

type Module struct {
	ID            int64       `json:"id,omitempty"`
	Plugin        string      `json:"plugin"`
	Model         string      `json:"model"`
	Version       string      `json:"version,omitempty"`
	Params        []Param     `json:"params,omitempty"`
	Pos           [2]int      `json:"pos,omitempty"`
	Bypass        bool        `json:"bypass,omitempty"`
	Disabled      bool        `json:"disabled,omitempty"`
	LeftModuleID  int64       `json:"leftModuleId,omitempty"`
	RightModuleID int64       `json:"rightModuleId,omitempty"`
	Data          interface{} `json:"data,omitempty"`
}

type Param struct {
	ID      int64   `json:"id,omitempty"`
	ParamID int64   `json:"paramId,omitempty"`
	Value   float64 `json:"value"`
}

type Cable struct {
	ID             int64  `json:"id,omitempty"`
	OutputModuleID int64  `json:"outputModuleId"`
	OutputID       int64  `json:"outputId"`
	InputModuleID  int64  `json:"inputModuleId"`
	InputID        int64  `json:"inputId"`
	Color          string `json:"color,omitempty"`
}
