package app

import "strconv"

var defaultPowerLevels = []PowerLevel{
	{Percent: 10, Label: "10 percent drive"},
	{Percent: 20, Label: "20 percent drive"},
	{Percent: 30, Label: "30 percent drive"},
	{Percent: 40, Label: "40 percent drive"},
	{Percent: 50, Label: "50 percent drive"},
	{Percent: 60, Label: "60 percent drive"},
	{Percent: 70, Label: "70 percent drive"},
	{Percent: 80, Label: "80 percent drive"},
	{Percent: 90, Label: "90 percent drive"},
	{Percent: 100, Label: "100 percent drive"},
}

func powerLevels() []PowerLevel {
	out := make([]PowerLevel, len(defaultPowerLevels))
	copy(out, defaultPowerLevels)
	return out
}

func powerLabelForPercent(percent int) string {
	for _, level := range defaultPowerLevels {
		if level.Percent == percent {
			return level.Label
		}
	}
	return clampLabel(percent)
}

func nextPowerLevel(current int) PowerLevel {
	current = clampPower(current)
	for _, level := range defaultPowerLevels {
		if level.Percent > current {
			return level
		}
	}
	return defaultPowerLevels[0]
}

func clampLabel(percent int) string {
	return powerLevelText(clampPower(percent))
}

func powerLevelText(percent int) string {
	return strconv.Itoa(percent) + " percent drive"
}
