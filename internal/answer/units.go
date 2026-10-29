package answer

import "fmt"

type unit struct {
	kind   string
	factor float64 // to the kind's base unit
	name   string  // how a result names it
}

// Base units: metre, kilogram, litre, byte, second, metre per second,
// square metre. Temperatures convert by formula.
var units = func() map[string]unit {
	m := map[string]unit{}
	add := func(kind string, factor float64, name string, words ...string) {
		for _, w := range words {
			m[w] = unit{kind, factor, name}
		}
	}
	add("length", 1e-3, "mm", "mm", "millimeter", "millimeters", "millimetre", "millimetres")
	add("length", 1e-2, "cm", "cm", "centimeter", "centimeters", "centimetre", "centimetres")
	add("length", 1, "m", "m", "meter", "meters", "metre", "metres")
	add("length", 1e3, "km", "km", "kilometer", "kilometers", "kilometre", "kilometres")
	add("length", 0.0254, "in", "in", "inch", "inches", `"`)
	add("length", 0.3048, "ft", "ft", "foot", "feet", "'")
	add("length", 0.9144, "yd", "yd", "yard", "yards")
	add("length", 1609.344, "mi", "mi", "mile", "miles")
	add("length", 1852, "nmi", "nmi", "nautical")

	add("mass", 1e-6, "mg", "mg", "milligram", "milligrams")
	add("mass", 1e-3, "g", "g", "gram", "grams", "gm")
	add("mass", 1, "kg", "kg", "kilo", "kilos", "kilogram", "kilograms", "kgs")
	add("mass", 1e3, "t", "tonne", "tonnes", "ton", "tons")
	add("mass", 0.028349523125, "oz", "oz", "ounce", "ounces")
	add("mass", 0.45359237, "lb", "lb", "lbs", "pound", "pounds")
	add("mass", 6.35029318, "st", "st", "stone", "stones")

	add("volume", 1e-3, "ml", "ml", "milliliter", "milliliters", "millilitre", "millilitres")
	add("volume", 1, "l", "l", "liter", "liters", "litre", "litres")
	add("volume", 3.785411784, "gal", "gal", "gallon", "gallons")
	add("volume", 0.946352946, "qt", "qt", "quart", "quarts")
	add("volume", 0.473176473, "pt", "pt", "pint", "pints")
	add("volume", 0.2365882365, "cups", "cup", "cups")
	add("volume", 0.0295735295625, "fl oz", "floz")
	add("volume", 0.01478676478125, "tbsp", "tbsp", "tablespoon", "tablespoons")
	add("volume", 0.00492892159375, "tsp", "tsp", "teaspoon", "teaspoons")

	add("data", 1.0/8, "bit", "bit", "bits")
	add("data", 1, "B", "b", "byte", "bytes")
	add("data", 1e3, "kB", "kb", "kilobyte", "kilobytes")
	add("data", 1e6, "MB", "mb", "megabyte", "megabytes")
	add("data", 1e9, "GB", "gb", "gigabyte", "gigabytes")
	add("data", 1e12, "TB", "tb", "terabyte", "terabytes")
	add("data", 1<<10, "KiB", "kib")
	add("data", 1<<20, "MiB", "mib")
	add("data", 1<<30, "GiB", "gib")
	add("data", 1<<40, "TiB", "tib")

	add("time", 1e-3, "ms", "ms", "millisecond", "milliseconds")
	add("time", 1, "s", "s", "sec", "secs", "second", "seconds")
	add("time", 60, "min", "min", "mins", "minute", "minutes")
	add("time", 3600, "h", "h", "hr", "hrs", "hour", "hours")
	add("time", 86400, "days", "d", "day", "days")
	add("time", 604800, "weeks", "wk", "week", "weeks")
	add("time", 31557600, "years", "yr", "year", "years")

	add("speed", 1, "m/s", "m/s", "mps")
	add("speed", 1/3.6, "km/h", "km/h", "kmh", "kph", "kmph")
	add("speed", 0.44704, "mph", "mph")
	add("speed", 0.514444, "kn", "kn", "knot", "knots")

	add("area", 1, "m²", "m2", "sqm", "m²")
	add("area", 1e6, "km²", "km2", "sqkm", "km²")
	add("area", 0.09290304, "ft²", "ft2", "sqft", "ft²")
	add("area", 1e4, "ha", "ha", "hectare", "hectares")
	add("area", 4046.8564224, "acres", "acre", "acres")

	add("temp", 0, "°C", "c", "°c", "celsius", "centigrade")
	add("temp", 0, "°F", "f", "°f", "fahrenheit")
	add("temp", 0, "K", "k", "kelvin")
	return m
}()

func toKelvin(v float64, name string) float64 {
	switch name {
	case "°C":
		return v + 273.15
	case "°F":
		return (v-32)*5/9 + 273.15
	}
	return v
}

func fromKelvin(v float64, name string) float64 {
	switch name {
	case "°C":
		return v - 273.15
	case "°F":
		return (v-273.15)*9/5 + 32
	}
	return v
}

// convertUnits answers "5 km to miles", "72f in c", "1.5 gb to mib".
func convertUnits(n float64, from, to string) (Answer, bool) {
	a, okA := units[from]
	b, okB := units[to]
	if !okA || !okB || a.kind != b.kind || a.name == b.name {
		return Answer{}, false
	}
	var v float64
	if a.kind == "temp" {
		v = fromKelvin(toKelvin(n, a.name), b.name)
	} else {
		v = n * a.factor / b.factor
	}
	value := Format(v)
	return Answer{
		Value: value,
		Name:  value + " " + b.name,
		Hint:  fmt.Sprintf("%s %s", Format(n), a.name),
	}, true
}
