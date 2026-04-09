package owntracks

import (
	"testing"
	"time"
)

func TestParseLocation(t *testing.T) {
	raw := []byte(`{"_type":"location","lat":52.5,"lon":13.4,"tst":1700000000,"acc":12.5}`)
	loc, err := ParseLocation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if loc.Lat != 52.5 || loc.Lon != 13.4 {
		t.Fatalf("lat/lon: %+v", loc)
	}
	want := time.Unix(1700000000, 0).UTC()
	if !loc.Tst.Equal(want) {
		t.Fatalf("tst: got %v want %v", loc.Tst, want)
	}
	if loc.Acc == nil || *loc.Acc != 12.5 {
		t.Fatalf("acc: %+v", loc.Acc)
	}
}

func TestParseLocationConnString(t *testing.T) {
	// OwnTracks 扩展数据：conn 为 "w"/"m"/"o"，不得按布尔解析（否则会 400）。
	raw := []byte(`{"_type":"location","lat":31.18,"lon":121.58,"tst":1700000000,"conn":"m","batt":85}`)
	_, err := ParseLocation(raw)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPayloadType(t *testing.T) {
	typ, err := PayloadType([]byte(`{"_type":"status","topic":"owntracks/u/d"}`))
	if err != nil || typ != "status" {
		t.Fatalf("got %q err=%v", typ, err)
	}
	typ, err = PayloadType([]byte(`{"_type":"location","lat":1,"lon":2,"tst":1}`))
	if err != nil || typ != "location" {
		t.Fatalf("got %q err=%v", typ, err)
	}
}

func TestSplitTopic(t *testing.T) {
	u, d, ok := SplitTopic("owntracks/alice/phone")
	if !ok || u != "alice" || d != "phone" {
		t.Fatalf("got %q %q %v", u, d, ok)
	}
}
