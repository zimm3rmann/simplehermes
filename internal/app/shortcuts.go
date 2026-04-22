package app

func shortcuts() []Shortcut {
	return []Shortcut{
		{Keys: "H", Description: "Read the shortcut list"},
		{Keys: "S", Description: "Open the settings dialog"},
		{Keys: "P", Description: "Cycle through the available power presets"},
		{Keys: "B", Description: "Cycle through the band presets"},
		{Keys: "Shift + B", Description: "Read the current band"},
		{Keys: "Shift + F", Description: "Read the current frequency"},
		{Keys: "M", Description: "Cycle through the operating modes"},
		{Keys: "Wheel", Description: "Tune up or down by the current step"},
		{Keys: "Arrow Up", Description: "Tune up by one current step"},
		{Keys: "Arrow Down", Description: "Tune down by one current step"},
		{Keys: "Shift + Arrow Up", Description: "Tune up by ten current steps"},
		{Keys: "Shift + Arrow Down", Description: "Tune down by ten current steps"},
		{Keys: "]", Description: "Tune up by one current step"},
		{Keys: "[", Description: "Tune down by one current step"},
		{Keys: "Shift + ]", Description: "Tune up by ten current steps"},
		{Keys: "Shift + [", Description: "Tune down by ten current steps"},
		{Keys: "R", Description: "Toggle receive"},
		{Keys: "T", Description: "Toggle transmit arm"},
		{Keys: "Hold Space", Description: "Key PTT while the key is held down"},
	}
}
