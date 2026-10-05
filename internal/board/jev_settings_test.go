package board

import "testing"

func TestJevProviderSettingsValidation(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	base := Defaults()
	ok := func(f func(*Settings)) Settings { v := base; f(&v); return v }
	for _, v := range []Settings{
		ok(func(v *Settings) { v.SmartAdd = true; v.JevProvider = "vercel" }),
		ok(func(v *Settings) {
			v.SmartAdd = true
			v.JevProvider = "cloudflare"
			v.JevCloudflareAccount = "0123456789abcdef0123456789abcdef"
		}),
		ok(func(v *Settings) {
			v.SmartAdd = true
			v.JevProvider = "custom"
			v.JevCustomURL = "https://jev.example.com"
			v.JevCustomModel = "jev"
		}),
		// Incomplete details may be saved while smart add is off.
		ok(func(v *Settings) { v.JevProvider = "cloudflare" }),
	} {
		if e := s.SaveSettings(v); e != nil {
			t.Fatalf("%+v: %v", v, e)
		}
	}
	for _, v := range []Settings{
		ok(func(v *Settings) { v.JevProvider = "unknown-provider" }),
		ok(func(v *Settings) { v.SmartAdd = true; v.JevProvider = "cloudflare" }),
		ok(func(v *Settings) { v.SmartAdd = true; v.JevProvider = "custom"; v.JevCustomURL = "http://jev.example.com"; v.JevCustomModel = "m" }),
	} {
		if e := s.SaveSettings(v); e == nil {
			t.Fatalf("%+v accepted", v)
		}
	}
	v := ok(func(v *Settings) { v.JevProvider = "custom"; v.JevCustomURL = "https://x.example"; v.JevCustomModel = "m" })
	if ep := v.JevEndpoint(); ep.Provider != "custom" || ep.BaseURL != "https://x.example" || ep.Model != "m" {
		t.Fatalf("%+v", ep)
	}
}
