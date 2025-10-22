package testutils

type WarpMenuCategory struct {
	Title   string
	Order   int
	Entries []WarpMenuEntry
}

type WarpMenuEntry struct {
	Title       string
	DisplayName string
	Href        string
	Target      string
}
