package app

import (
	"os/exec"
	"testing"
)

// A path with an accent in it has to come back from a Windows dialog as it
// was chosen. PowerShell prints to a pipe in the console's code page, and
// "poupée.pdf" came back with one byte where the é was, which opened nothing.
func TestPowershellSaysAccentsWhole(t *testing.T) {
	const path = `C:\Users\utilisateur\Desktop\Matriochka partitions et livret\2)Petite poupée.pdf`

	said, err := powershellSaid([]byte("QzpcVXNlcnNcdXRpbGlzYXRldXJcRGVza3RvcFxNYXRyaW9jaGthIHBhcnRpdGlvbnMgZXQgbGl2cmV0XDIpUGV0aXRlIHBvdXDDqWUucGRm\r\n"))
	if err != nil || said != path {
		t.Fatalf("read back %q, %v; want %q", said, err, path)
	}
	if said, err := powershellSaid(nil); err != nil || said != "" {
		t.Errorf("a closed panel read back as %q, %v; want nothing", said, err)
	}
	if _, err := powershellSaid([]byte("poup\x82e.pdf")); err == nil {
		t.Error("a path printed plainly was taken for an answer")
	}

	// The whole trip, where there is a PowerShell to make it with, and with
	// the console set to the code page a French Windows has.
	shell := ""
	for _, name := range []string{"powershell", "pwsh"} {
		if p, err := exec.LookPath(name); err == nil {
			shell = p
			break
		}
	}
	if shell == "" {
		t.Skip("no PowerShell here")
	}
	// Not only French: no code page holds all of these at once, and the
	// last is outside the plane that UTF-16 writes in one unit.
	want := []string{
		path,
		`D:\été\l'île.pdf`,
		`C:\Users\用户\桌面\乐谱\小娃娃.pdf`,
		`C:\Users\Пользователь\Ноты\Матрёшка.gscore`,
		`C:\Users\مستخدم\楽譜\악보 𝄞.musicxml`,
	}
	list := ""
	for i, p := range want {
		if i > 0 {
			list += ", "
		}
		list += "'" + powershellQuote(p) + "'"
	}
	script := `[Console]::OutputEncoding = [Text.Encoding]::GetEncoding(850);` +
		`$f = @(` + list + `);` +
		powershellSay("($f -join \"`n\")")
	out, err := exec.Command(shell, "-NoProfile", "-Command", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	said, err = powershellSaid(out)
	if err != nil {
		t.Fatal(err)
	}
	got := pathLines(said)
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path %d came back as %q, want %q", i+1, got[i], want[i])
		}
	}
}
