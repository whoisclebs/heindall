package fraud

import "bytes"

func VectorizeJSON(data []byte, norm Normalization, mccRisk map[string]float64) ([Dimensions]float32, bool) {
	var vec [Dimensions]float32

	amount, ok := numberAfter(data, []byte(`"amount"`), 0)
	if !ok {
		return vec, false
	}
	installments, ok := numberAfter(data, []byte(`"installments"`), 0)
	if !ok {
		return vec, false
	}
	requestedAt, ok := stringAfter(data, []byte(`"requested_at"`), 0)
	if !ok || len(requestedAt) < 20 {
		return vec, false
	}

	customerPos := bytes.Index(data, []byte(`"customer"`))
	merchantPos := bytes.Index(data, []byte(`"merchant"`))
	terminalPos := bytes.Index(data, []byte(`"terminal"`))
	if customerPos < 0 || merchantPos < 0 || terminalPos < 0 {
		return vec, false
	}

	customerAvg, ok := numberAfter(data, []byte(`"avg_amount"`), customerPos)
	if !ok {
		return vec, false
	}
	txCount, ok := numberAfter(data, []byte(`"tx_count_24h"`), customerPos)
	if !ok {
		return vec, false
	}
	merchantID, ok := stringAfter(data, []byte(`"id"`), merchantPos)
	if !ok {
		return vec, false
	}
	mcc, ok := stringAfter(data, []byte(`"mcc"`), merchantPos)
	if !ok {
		return vec, false
	}
	merchantAvg, ok := numberAfter(data, []byte(`"avg_amount"`), merchantPos)
	if !ok {
		return vec, false
	}
	isOnline, ok := boolAfter(data, []byte(`"is_online"`), terminalPos)
	if !ok {
		return vec, false
	}
	cardPresent, ok := boolAfter(data, []byte(`"card_present"`), terminalPos)
	if !ok {
		return vec, false
	}
	kmFromHome, ok := numberAfter(data, []byte(`"km_from_home"`), terminalPos)
	if !ok {
		return vec, false
	}

	vec[0] = f32(clamp(div(amount, norm.MaxAmount)))
	vec[1] = f32(clamp(div(installments, norm.MaxInstallments)))
	vec[2] = f32(clamp(div(div(amount, customerAvg), norm.AmountVsAvgRatio)))
	vec[3] = f32(float64(parse2(requestedAt[11:13])) / 23.0)
	vec[4] = f32(float64(weekdayMonday0(requestedAt)) / 6.0)
	vec[7] = f32(clamp(div(kmFromHome, norm.MaxKM)))
	vec[8] = f32(clamp(div(txCount, norm.MaxTxCount24h)))
	vec[9] = boolFloat(isOnline)
	vec[10] = boolFloat(cardPresent)
	vec[11] = boolFloat(!knownMerchantJSON(data, merchantID, customerPos, merchantPos))
	vec[12] = f32(mccRiskValueBytes(mcc, mccRisk))
	vec[13] = f32(clamp(div(merchantAvg, norm.MaxMerchantAvgAmount)))

	lastPos := bytes.Index(data, []byte(`"last_transaction"`))
	if lastPos < 0 || bytes.HasPrefix(skipValuePrefix(data[lastPos+len(`"last_transaction"`):]), []byte("null")) {
		vec[5] = -1
		vec[6] = -1
		return vec, true
	}
	lastTimestamp, ok := stringAfter(data, []byte(`"timestamp"`), lastPos)
	if !ok || len(lastTimestamp) < 20 {
		return vec, false
	}
	kmFromCurrent, ok := numberAfter(data, []byte(`"km_from_current"`), lastPos)
	if !ok {
		return vec, false
	}
	minutes := float64(epochMinutes(requestedAt) - epochMinutes(lastTimestamp))
	vec[5] = f32(clamp(div(minutes, norm.MaxMinutes)))
	vec[6] = f32(clamp(div(kmFromCurrent, norm.MaxKM)))

	return vec, true
}

func VectorizeJSONQuantized(data []byte, norm Normalization) ([Dimensions]int16, bool) {
	var vec [Dimensions]int16

	amount, ok := numberAfter(data, []byte(`"amount"`), 0)
	if !ok {
		return vec, false
	}
	installments, ok := numberAfter(data, []byte(`"installments"`), 0)
	if !ok {
		return vec, false
	}
	requestedAt, ok := stringAfter(data, []byte(`"requested_at"`), 0)
	if !ok || len(requestedAt) < 20 {
		return vec, false
	}

	customerPos := bytes.Index(data, []byte(`"customer"`))
	merchantPos := bytes.Index(data, []byte(`"merchant"`))
	terminalPos := bytes.Index(data, []byte(`"terminal"`))
	if customerPos < 0 || merchantPos < 0 || terminalPos < 0 {
		return vec, false
	}

	customerAvg, ok := numberAfter(data, []byte(`"avg_amount"`), customerPos)
	if !ok {
		return vec, false
	}
	txCount, ok := numberAfter(data, []byte(`"tx_count_24h"`), customerPos)
	if !ok {
		return vec, false
	}
	merchantID, ok := stringAfter(data, []byte(`"id"`), merchantPos)
	if !ok {
		return vec, false
	}
	mcc, ok := stringAfter(data, []byte(`"mcc"`), merchantPos)
	if !ok {
		return vec, false
	}
	merchantAvg, ok := numberAfter(data, []byte(`"avg_amount"`), merchantPos)
	if !ok {
		return vec, false
	}
	isOnline, ok := boolAfter(data, []byte(`"is_online"`), terminalPos)
	if !ok {
		return vec, false
	}
	cardPresent, ok := boolAfter(data, []byte(`"card_present"`), terminalPos)
	if !ok {
		return vec, false
	}
	kmFromHome, ok := numberAfter(data, []byte(`"km_from_home"`), terminalPos)
	if !ok {
		return vec, false
	}

	vec[0] = quantizeFloat64(clamp(div(amount, norm.MaxAmount)))
	vec[1] = quantizeFloat64(clamp(div(installments, norm.MaxInstallments)))
	vec[2] = quantizeFloat64(clamp(div(div(amount, customerAvg), norm.AmountVsAvgRatio)))
	vec[3] = quantizeFloat64(float64(parse2(requestedAt[11:13])) / 23.0)
	vec[4] = quantizeFloat64(float64(weekdayMonday0(requestedAt)) / 6.0)
	vec[7] = quantizeFloat64(clamp(div(kmFromHome, norm.MaxKM)))
	vec[8] = quantizeFloat64(clamp(div(txCount, norm.MaxTxCount24h)))
	vec[9] = quantizeBool(isOnline)
	vec[10] = quantizeBool(cardPresent)
	vec[11] = quantizeBool(!knownMerchantJSON(data, merchantID, customerPos, merchantPos))
	vec[12] = quantizeFloat64(mccRiskValueBytesFast(mcc))
	vec[13] = quantizeFloat64(clamp(div(merchantAvg, norm.MaxMerchantAvgAmount)))

	lastPos := bytes.Index(data, []byte(`"last_transaction"`))
	if lastPos < 0 || bytes.HasPrefix(skipValuePrefix(data[lastPos+len(`"last_transaction"`):]), []byte("null")) {
		vec[5] = -QuantScale
		vec[6] = -QuantScale
		return vec, true
	}
	lastTimestamp, ok := stringAfter(data, []byte(`"timestamp"`), lastPos)
	if !ok || len(lastTimestamp) < 20 {
		return vec, false
	}
	kmFromCurrent, ok := numberAfter(data, []byte(`"km_from_current"`), lastPos)
	if !ok {
		return vec, false
	}
	minutes := float64(epochMinutes(requestedAt) - epochMinutes(lastTimestamp))
	vec[5] = quantizeFloat64(clamp(div(minutes, norm.MaxMinutes)))
	vec[6] = quantizeFloat64(clamp(div(kmFromCurrent, norm.MaxKM)))

	return vec, true
}

func numberAfter(data, key []byte, start int) (float64, bool) {
	idx := bytes.Index(data[start:], key)
	if idx < 0 {
		return 0, false
	}
	pos := start + idx + len(key)
	for pos < len(data) && (data[pos] == ':' || data[pos] <= ' ') {
		pos++
	}
	end := pos
	for end < len(data) && ((data[end] >= '0' && data[end] <= '9') || data[end] == '.' || data[end] == '-') {
		end++
	}
	if end == pos {
		return 0, false
	}
	return parseFloatBytes(data[pos:end])
}

func parseFloatBytes(data []byte) (float64, bool) {
	if len(data) == 0 {
		return 0, false
	}
	neg := false
	pos := 0
	if data[0] == '-' {
		neg = true
		pos++
		if pos == len(data) {
			return 0, false
		}
	}
	var value float64
	seenDigit := false
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		seenDigit = true
		value = value*10 + float64(data[pos]-'0')
		pos++
	}
	if pos < len(data) && data[pos] == '.' {
		pos++
		factor := 0.1
		for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
			seenDigit = true
			value += float64(data[pos]-'0') * factor
			factor *= 0.1
			pos++
		}
	}
	if !seenDigit || pos != len(data) {
		return 0, false
	}
	if neg {
		value = -value
	}
	return value, true
}

func stringAfter(data, key []byte, start int) ([]byte, bool) {
	idx := bytes.Index(data[start:], key)
	if idx < 0 {
		return nil, false
	}
	pos := start + idx + len(key)
	for pos < len(data) && data[pos] != '"' {
		pos++
	}
	pos++
	end := pos
	for end < len(data) && data[end] != '"' {
		end++
	}
	if end > len(data) || pos > end {
		return nil, false
	}
	return data[pos:end], true
}

func boolAfter(data, key []byte, start int) (bool, bool) {
	idx := bytes.Index(data[start:], key)
	if idx < 0 {
		return false, false
	}
	pos := start + idx + len(key)
	for pos < len(data) && (data[pos] == ':' || data[pos] <= ' ') {
		pos++
	}
	if bytes.HasPrefix(data[pos:], []byte("true")) {
		return true, true
	}
	if bytes.HasPrefix(data[pos:], []byte("false")) {
		return false, true
	}
	return false, false
}

func knownMerchantJSON(data, merchantID []byte, start, end int) bool {
	if start < 0 || start >= len(data) {
		return false
	}
	if end <= start || end > len(data) {
		end = len(data)
	}
	knownPos := bytes.Index(data[start:end], []byte(`"known_merchants"`))
	if knownPos < 0 {
		return false
	}
	sectionStart := start + knownPos
	sectionEnd := bytes.IndexByte(data[sectionStart:end], ']')
	if sectionEnd < 0 {
		return false
	}
	section := data[sectionStart : sectionStart+sectionEnd]
	searchStart := 0
	for {
		idx := bytes.Index(section[searchStart:], merchantID)
		if idx < 0 {
			return false
		}
		pos := searchStart + idx
		if pos > 0 && section[pos-1] == '"' && pos+len(merchantID) < len(section) && section[pos+len(merchantID)] == '"' {
			return true
		}
		searchStart = pos + 1
	}
}

func mccRiskValueBytes(mcc []byte, risk map[string]float64) float64 {
	if v, ok := mccRiskValueBytesKnown(mcc); ok {
		return v
	}
	return 0.5
}

func mccRiskValueBytesFast(mcc []byte) float64 {
	if v, ok := mccRiskValueBytesKnown(mcc); ok {
		return v
	}
	return 0.5
}

func mccRiskValueBytesKnown(mcc []byte) (float64, bool) {
	if len(mcc) != 4 {
		return 0, false
	}
	switch uint32(mcc[0])<<24 | uint32(mcc[1])<<16 | uint32(mcc[2])<<8 | uint32(mcc[3]) {
	case '5'<<24 | '4'<<16 | '1'<<8 | '1':
		return 0.15, true
	case '5'<<24 | '8'<<16 | '1'<<8 | '2':
		return 0.30, true
	case '5'<<24 | '9'<<16 | '1'<<8 | '2':
		return 0.20, true
	case '5'<<24 | '9'<<16 | '4'<<8 | '4':
		return 0.45, true
	case '7'<<24 | '8'<<16 | '0'<<8 | '1':
		return 0.80, true
	case '7'<<24 | '8'<<16 | '0'<<8 | '2':
		return 0.75, true
	case '7'<<24 | '9'<<16 | '9'<<8 | '5':
		return 0.85, true
	case '4'<<24 | '5'<<16 | '1'<<8 | '1':
		return 0.35, true
	case '5'<<24 | '3'<<16 | '1'<<8 | '1':
		return 0.25, true
	case '5'<<24 | '9'<<16 | '9'<<8 | '9':
		return 0.50, true
	default:
		return 0, false
	}
}

func quantizeFloat64(value float64) int16 {
	if value <= -1 {
		return -QuantScale
	}
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return int16(value*float64(QuantScale) + 0.5)
}

func quantizeBool(value bool) int16 {
	if value {
		return QuantScale
	}
	return 0
}

func skipValuePrefix(data []byte) []byte {
	for len(data) > 0 && (data[0] == ':' || data[0] <= ' ') {
		data = data[1:]
	}
	return data
}

func parse2(data []byte) int { return int(data[0]-'0')*10 + int(data[1]-'0') }

func parse4(data []byte) int { return parse2(data[:2])*100 + parse2(data[2:4]) }

func weekdayMonday0(ts []byte) int {
	y := parse4(ts[0:4])
	m := parse2(ts[5:7])
	d := parse2(ts[8:10])
	return int((daysFromCivil(y, m, d) + 3) % 7)
}

func epochMinutes(ts []byte) int64 {
	y := parse4(ts[0:4])
	m := parse2(ts[5:7])
	d := parse2(ts[8:10])
	hh := parse2(ts[11:13])
	mm := parse2(ts[14:16])
	return daysFromCivil(y, m, d)*1440 + int64(hh*60+mm)
}

func daysFromCivil(y, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := divFloor(y, 400)
	yoe := y - era*400
	mp := m + 9
	if m > 2 {
		mp = m - 3
	}
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return int64(era*146097 + doe - 719468)
}

func divFloor(a, b int) int {
	q := a / b
	r := a % b
	if r != 0 && ((r < 0) != (b < 0)) {
		q--
	}
	return q
}
