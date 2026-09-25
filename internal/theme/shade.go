package theme

import (
	"fmt"
	"math"
)

// Shade returns the ANSI escape sequence for the color of a role with its HSV
// value shifted by valueShift. The hue and saturation of the role stay as they
// are, and the value is clamped to 0..1. It returns an empty string when
// NO_COLOR is set or the role is unknown.
func (t *Theme) Shade(role string, valueShift float64) string {
	if t.isColorless {
		return ""
	}
	hex, ok := t.colors[role]
	if !ok {
		return ""
	}
	hue, saturation, value := rgbToHSV(hexToRGB(hex))
	shiftedValue := math.Max(0, math.Min(1, value+valueShift))
	red, green, blue := hsvToRGB(hue, saturation, shiftedValue)
	return fmt.Sprintf(ansiColorPrefix, red, green, blue)
}

// rgbToHSV returns the hue in degrees and the saturation and value in 0..1.
func rgbToHSV(red, green, blue int) (hue, saturation, value float64) {
	redPart, greenPart, bluePart := float64(red)/255, float64(green)/255, float64(blue)/255
	high := math.Max(redPart, math.Max(greenPart, bluePart))
	low := math.Min(redPart, math.Min(greenPart, bluePart))
	delta := high - low
	value = high
	if high > 0 {
		saturation = delta / high
	}
	switch {
	case delta == 0:
		hue = 0
	case high == redPart:
		hue = math.Mod((greenPart-bluePart)/delta, 6)
	case high == greenPart:
		hue = (bluePart-redPart)/delta + 2
	default:
		hue = (redPart-greenPart)/delta + 4
	}
	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue, saturation, value
}

func hsvToRGB(hue, saturation, value float64) (red, green, blue int) {
	chroma := value * saturation
	secondary := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	offset := value - chroma
	var redPart, greenPart, bluePart float64
	switch {
	case hue < 60:
		redPart, greenPart, bluePart = chroma, secondary, 0
	case hue < 120:
		redPart, greenPart, bluePart = secondary, chroma, 0
	case hue < 180:
		redPart, greenPart, bluePart = 0, chroma, secondary
	case hue < 240:
		redPart, greenPart, bluePart = 0, secondary, chroma
	case hue < 300:
		redPart, greenPart, bluePart = secondary, 0, chroma
	default:
		redPart, greenPart, bluePart = chroma, 0, secondary
	}
	toByte := func(part float64) int { return int(math.Round((part + offset) * 255)) }
	return toByte(redPart), toByte(greenPart), toByte(bluePart)
}
