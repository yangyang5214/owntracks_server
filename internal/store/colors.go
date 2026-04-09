package store

// Palette 区分成员用色（高对比、暗色地图上可读）。
var Palette = []string{
	"#FF6B9D", "#4ECDC4", "#FFE66D", "#A78BFA", "#67E8F9",
	"#FBBF24", "#34D399", "#F472B6", "#60A5FA",
}

func colorForIndex(i int) string {
	if i < 0 {
		i = 0
	}
	return Palette[i%len(Palette)]
}
