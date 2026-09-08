package gsm

// DefaultPresets returns fresh copies of the five built-in configurations.
// They are written only when creating a new store or migrating a version-1
// store; a version-2 store is authoritative, including edits and deletions.
func DefaultPresets() []Preset {
	return []Preset{
		defaultPreset("0", "addx", "1800", "540", "001", "01"),
		defaultPreset("1", "ChinaMobile", "900", "55", "460", "00"),
		defaultPreset("2", "ChinaMobile", "1800", "540", "460", "00"),
		defaultPreset("3", "ChinaUnicom", "900", "70", "460", "01"),
		defaultPreset("4", "ChinaUnicom", "1800", "668", "460", "01"),
	}
}

func defaultPreset(id, name, band, c0, mcc, mnc string) Preset {
	return Preset{
		ID:   id,
		Name: name,
		Params: StartParams{
			ARFCNs: "1", C0: c0, Band: band, MCC: mcc, MNC: mnc,
			LAC: "1", CI: "1", ShortName: name, Network: "eth0",
		},
	}
}
