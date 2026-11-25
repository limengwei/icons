package importer

import (
	"reflect"
	"testing"
)

// sampleManifest mimics the real lib/iconsManifest.js layout (first entries
// copied from it), with bracket characters inside strings, a "lib" entry to
// skip and trailing code with stray brackets after the array.
const sampleManifest = `module.exports.IconsManifest = [
  {
    "id": "ci",
    "name": "Circum Icons [2024]",
    "projectUrl": "https://circumicons.com/",
    "license": "MPL-2.0 license",
    "licenseUrl": "https://github.com/Klarr-Agency/Circum-Icons/blob/main/LICENSE"
  },
  {
    "id": "fa",
    "name": "Font Awesome 5",
    "projectUrl": "https://fontawesome.com/",
    "license": "CC BY 4.0 License",
    "licenseUrl": "https://creativecommons.org/licenses/by/4.0/"
  },
  {
    "id": "lib",
    "name": "lib [not] a set",
    "projectUrl": "",
    "license": "",
    "licenseUrl": ""
  }
];
module.exports.other = "trailing ] bracket [ noise";
`

func TestParseManifest(t *testing.T) {
	sets, err := ParseManifest(sampleManifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	want := []Set{
		{
			ID:         "ci",
			Name:       "Circum Icons [2024]",
			ProjectURL: "https://circumicons.com/",
			License:    "MPL-2.0 license",
			LicenseURL: "https://github.com/Klarr-Agency/Circum-Icons/blob/main/LICENSE",
			Src:        SourceReactIcons,
		},
		{
			ID:         "fa",
			Name:       "Font Awesome 5",
			ProjectURL: "https://fontawesome.com/",
			License:    "CC BY 4.0 License",
			LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
			Src:        SourceReactIcons,
		},
	}
	if !reflect.DeepEqual(sets, want) {
		t.Errorf("sets = %+v, want %+v", sets, want)
	}
}

func TestParseManifestErrors(t *testing.T) {
	for name, src := range map[string]string{
		"no marker":     `module.exports.Other = [];`,
		"no bracket":    `module.exports.IconsManifest = 42;`,
		"unbalanced":    `module.exports.IconsManifest = [ {"id":"fa"}`,
		"truncated str": `module.exports.IconsManifest = [ {"id":"fa]`,
	} {
		if _, err := ParseManifest(src); err == nil {
			t.Errorf("%s: ParseManifest succeeded, want error", name)
		}
	}
}

func TestSplitName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"FaYoutubeSquare", "fa youtube square"},
		{"Fa500px", "fa 500px"},
		{"Si500px", "si 500px"},
		{"IoIosAdd", "io ios add"},
		{"GoArrowDown", "go arrow down"},
		{"FaBeer", "fa beer"},
		{"FaCss3Alt", "fa css 3 alt"},
		{"SiHtml5", "si html 5"},
		{"FaCSS3", "fa css 3"},
		{"FaHTTPSClient", "fa https client"},
		// Iconify kebab-case names: hyphens separate words.
		{"home-minimal", "home minimal"},
		{"4k-bold-duotone", "4k bold duotone"},
		{"user--rounded", "user  rounded"},
	}
	for _, tt := range tests {
		if got := SplitName(tt.name); got != tt.want {
			t.Errorf("SplitName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSearchText(t *testing.T) {
	tests := []struct {
		name, setID, setName, want string
	}{
		{
			"FaBeer", "fa6", "Font Awesome 6",
			"fabeer fa beer fa6 font awesome 6",
		},
		{
			// "fa" already appears as a SplitName word, so the set ID
			// dedupes away.
			"FaBeer", "fa", "Font Awesome",
			"fabeer fa beer font awesome",
		},
		{
			"Si500px", "si", "Simple Icons",
			"si500px si 500px simple icons",
		},
		{
			"GoGithub", "go", "Github Octicons icons",
			"gogithub go github octicons icons",
		},
		{
			"Gi3DGlasses", "gi", "Game Icons",
			"gi3dglasses gi 3 d glasses game icons",
		},
	}
	for _, tt := range tests {
		if got := SearchText(tt.name, tt.setID, tt.setName); got != tt.want {
			t.Errorf("SearchText(%q, %q, %q) = %q, want %q",
				tt.name, tt.setID, tt.setName, got, tt.want)
		}
	}
}
