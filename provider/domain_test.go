package provider

import (
	"reflect"
	"testing"
)

func TestParseDkimRecords(t *testing.T) {
	// Mirrors mox's zone-file output: explanatory comment lines precede a
	// long (multi-chunk, parenthesised) DKIM record, plus unrelated records.
	records := []string{
		"; Deliver email for the domain to this host.",
		"example.com.                 MX 10 mail.example.com.",
		"; NOTE: The following is a single long record split over several lines for use",
		"; in zone files. When adding through a DNS operator web interface, combine the",
		"; strings into a single string, without ().",
		"2026a._domainkey.example.com.   TXT (\n \t\t\"v=DKIM1;h=sha256;p=AAAA\"\n \t\t\"BBBBCCCC\"\n\t)",
		"2026b._domainkey.example.com.   TXT \"v=DKIM1;h=sha256;p=SHORTKEY\"",
		"example.com.                 TXT \"v=spf1 ip4:1.2.3.4 ~all\"",
		"_dmarc.example.com.          TXT \"v=DMARC1;p=reject;rua=mailto:dmarcreports@example.com!10m\"",
	}

	got := parseDkimRecords(records)
	want := []DkimDNSRecord{
		{Selector: "2026a", Txt: "v=DKIM1;h=sha256;p=AAAABBBBCCCC"},
		{Selector: "2026b", Txt: "v=DKIM1;h=sha256;p=SHORTKEY"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDkimRecords:\n got=%#v\nwant=%#v", got, want)
	}
}

func TestParseDkimRecordsNone(t *testing.T) {
	if got := parseDkimRecords([]string{"example.com. MX 10 mail.example.com."}); len(got) != 0 {
		t.Fatalf("expected no DKIM records, got %#v", got)
	}
}
