package modes

type Preset struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

var presets = []Preset{
	{ID: "lsb", Label: "LSB", Description: "Lower sideband voice"},
	{ID: "usb", Label: "USB", Description: "Upper sideband voice"},
	{ID: "cw", Label: "CW", Description: "Continuous wave"},
	{ID: "am", Label: "AM", Description: "Amplitude modulation"},
	{ID: "fm", Label: "FM", Description: "Frequency modulation"},
	{ID: "digu", Label: "DIGU", Description: "Digital upper sideband"},
}

func All() []Preset {
	out := make([]Preset, len(presets))
	copy(out, presets)
	return out
}

func ByID(id string) (Preset, bool) {
	for _, preset := range presets {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}

func Default() Preset {
	return presets[1]
}

func Next(currentID string) Preset {
	for index, preset := range presets {
		if preset.ID == currentID {
			return presets[(index+1)%len(presets)]
		}
	}
	return presets[0]
}
