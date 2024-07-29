package oklink

import (
	"encoding/base64"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const apiKey string = "a2c903cc-b31e-4547-9299-b6d07b7631ab"
const a int64 = 1111111111111

func GetApiKey() string {
	t := time.Now().UnixNano() / int64(time.Millisecond)
	v1 := EncryptApikey()
	v2 := EncryptTime(t)
	return comb(v1, v2)
}

func EncryptApikey() string {
	t := apiKey
	e := strings.Split(t, "")
	r := e[:8]
	e = e[8:]
	e = append(e, r...)
	return strings.Join(e, "")
}

func EncryptTime(t int64) string {
	e := strconv.FormatInt(t+a, 10)
	r := rand.Intn(10)
	n := rand.Intn(10)
	i := rand.Intn(10)
	return e + strconv.Itoa(r) + strconv.Itoa(n) + strconv.Itoa(i)
}

func comb(t, e string) string {
	r := t + "|" + e
	return base64.StdEncoding.EncodeToString([]byte(r))
}
