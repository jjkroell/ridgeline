// Package radio parses and compares the LoRa PHY config string that observers
// report and operators declare: "freq,bw,sf,cr" (MHz, kHz, spreading factor,
// coding rate).
//
// Two receivers are on the same RF segment when they can hear each other, which
// is a question about the PHY and not about how the numbers were typed. The
// same 910.425 MHz channel arrives from the field as both "910.4249877" (a
// radio reporting its synthesised centre frequency) and "910.425" (the same
// channel, rounded), and an operator declaring a bridge's far side writes
// "909.000" where the receiver on that side reports "909.0". Comparing those as
// strings splits one network into several, which is exactly the mistake that
// makes a segment-aware detector misclassify nodes.
package radio

import (
	"math"
	"strconv"
	"strings"
)

// freqTolMHz is how far apart two frequencies may be and still count as the
// same channel. 5 kHz absorbs both synthesiser rounding and hand-typed
// declarations while staying far below any real channel spacing (the narrowest
// bandwidth in use here is 62.5 kHz).
const freqTolMHz = 0.005

// bwTolKHz likewise absorbs formatting, not real differences.
const bwTolKHz = 0.01

// Profile is a parsed radio config. Fields are only meaningful when the
// corresponding Has* flag is set: observers report all four, but an operator
// declaring a far segment may type only a frequency.
type Profile struct {
	FreqMHz float64
	BWkHz   float64
	SF      int
	CR      int

	HasFreq bool
	HasBW   bool
	HasSF   bool
	HasCR   bool
}

// Parse reads a "freq,bw,sf,cr" string. Missing or unparseable components are
// left unset rather than defaulted — a zero frequency would compare equal to
// another zero frequency and silently merge two unknowns.
func Parse(s string) Profile {
	var p Profile
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) >= 1 {
		if f, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
			p.FreqMHz, p.HasFreq = f, true
		}
	}
	if len(parts) >= 2 {
		if f, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
			p.BWkHz, p.HasBW = f, true
		}
	}
	if len(parts) >= 3 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
			p.SF, p.HasSF = n, true
		}
	}
	if len(parts) >= 4 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil {
			p.CR, p.HasCR = n, true
		}
	}
	return p
}

// SameSegment reports whether two configs describe one RF network — receivers
// that can hear each other's traffic.
//
// Frequency, bandwidth and spreading factor must agree: they define the channel
// and the modulation, and a mismatch in any of them means no reception. CODING
// RATE IS DELIBERATELY IGNORED. LoRa carries CR in the explicit packet header,
// so a receiver decodes a sender's rate whatever its own is set to — splitting
// on it would put a station configured 4/8 on its own island despite it sitting
// in the middle of the same mesh.
//
// An unknown component on EITHER side is not a match. "Probably the same" is
// how a near-side receiver ends up classified as far-side, and the caller can
// always fall back to the traffic-based test.
func (p Profile) SameSegment(q Profile) bool {
	if !p.HasFreq || !q.HasFreq || math.Abs(p.FreqMHz-q.FreqMHz) > freqTolMHz {
		return false
	}
	if !p.HasBW || !q.HasBW || math.Abs(p.BWkHz-q.BWkHz) > bwTolKHz {
		return false
	}
	if !p.HasSF || !q.HasSF || p.SF != q.SF {
		return false
	}
	return true
}

// SameSegmentString is SameSegment over two raw config strings.
func SameSegmentString(a, b string) bool { return Parse(a).SameSegment(Parse(b)) }

// Normalize returns the config string with its FREQUENCY rounded to kHz
// precision, so the one channel reported as 910.4249877 and 910.425 stores and
// displays as a single value.
//
// Only the frequency is rewritten. Bandwidth, spreading factor and coding rate
// arrive in a stable form and reformatting them would churn every observer row
// for no gain. An unparseable string is returned unchanged: this runs over
// stored data, and mangling a value nobody can interpret is worse than keeping
// it verbatim for someone to look at.
func Normalize(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	parts := strings.Split(t, ",")
	f, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return t
	}
	parts[0] = FormatMHz(f)
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ",")
}

// FormatMHz renders a frequency at kHz precision, always with a decimal point
// so it still reads as a frequency: 910.4249877 -> "910.425", 909 -> "909.0".
func FormatMHz(f float64) string {
	s := strconv.FormatFloat(math.Round(f*1000)/1000, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// String renders a profile back to "freq,bw,sf,cr", omitting unset trailing
// components.
func (p Profile) String() string {
	if !p.HasFreq {
		return ""
	}
	out := FormatMHz(p.FreqMHz)
	if !p.HasBW {
		return out
	}
	out += "," + strconv.FormatFloat(p.BWkHz, 'f', -1, 64)
	if !p.HasSF {
		return out
	}
	out += "," + strconv.Itoa(p.SF)
	if !p.HasCR {
		return out
	}
	return out + "," + strconv.Itoa(p.CR)
}
