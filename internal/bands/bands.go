package bands

type Preset struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	DefaultHz int64  `json:"defaultHz"`
	MinHz     int64  `json:"minHz"`
	MaxHz     int64  `json:"maxHz"`
	TXAllowed bool   `json:"txAllowed"`
}

var presets = []Preset{
	{ID: "160m", Label: "160 m", DefaultHz: 1_900_000, MinHz: 1_800_000, MaxHz: 2_000_000, TXAllowed: true},
	{ID: "80m", Label: "80 m", DefaultHz: 3_850_000, MinHz: 3_500_000, MaxHz: 4_000_000, TXAllowed: true},
	{ID: "60m", Label: "60 m", DefaultHz: 5_357_000, MinHz: 5_261_250, MaxHz: 5_408_000, TXAllowed: true},
	{ID: "40m", Label: "40 m", DefaultHz: 7_200_000, MinHz: 7_000_000, MaxHz: 7_300_000, TXAllowed: true},
	{ID: "30m", Label: "30 m", DefaultHz: 10_125_000, MinHz: 10_100_000, MaxHz: 10_150_000, TXAllowed: true},
	{ID: "20m", Label: "20 m", DefaultHz: 14_200_000, MinHz: 14_000_000, MaxHz: 14_350_000, TXAllowed: true},
	{ID: "17m", Label: "17 m", DefaultHz: 18_120_000, MinHz: 18_068_000, MaxHz: 18_168_000, TXAllowed: true},
	{ID: "15m", Label: "15 m", DefaultHz: 21_300_000, MinHz: 21_000_000, MaxHz: 21_450_000, TXAllowed: true},
	{ID: "12m", Label: "12 m", DefaultHz: 24_940_000, MinHz: 24_890_000, MaxHz: 24_990_000, TXAllowed: true},
	{ID: "10m", Label: "10 m", DefaultHz: 28_400_000, MinHz: 28_000_000, MaxHz: 29_700_000, TXAllowed: true},
	{ID: "6m", Label: "6 m", DefaultHz: 50_125_000, MinHz: 50_000_000, MaxHz: 54_000_000, TXAllowed: true},
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

func ForFrequency(hz int64) (Preset, bool) {
	for _, preset := range presets {
		if hz >= preset.MinHz && hz <= preset.MaxHz {
			return preset, true
		}
	}
	return Preset{}, false
}

func Next(currentID string) Preset {
	for index, preset := range presets {
		if preset.ID == currentID {
			return presets[(index+1)%len(presets)]
		}
	}
	return presets[0]
}
