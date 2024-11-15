package ipxe

import (
	_ "embed"
	"fmt"
)

type MenuItem struct {
	Title string
	Link  string
	Args  string
}

type Menu struct {
	Title string
	Id    string
	Items []MenuItem

	Timeout string
	Default string
	Cancel  string
}

func (m *Menu) AddItem(title, link, args string) {
	m.Items = append(m.Items, MenuItem{title, link, args})
}

func (m *Menu) PrintTo(script *IPXEScript) {
	script.Append(fmt.Sprintf(":%s\nmenu %s\n", m.Id, m.Title))
	for _, item := range m.Items {
		script.Append(fmt.Sprintf("item %s %s %s\n", item.Args, item.Link, item.Title))
	}
	str := "choose "
	if m.Timeout != "" {
		str += fmt.Sprintf("--timeout %s ", m.Timeout)
	}
	if m.Default != "" {
		str += fmt.Sprintf("--default %s ", m.Default)
	}

	script.Append(str + fmt.Sprintf(" target-%s || goto %s\n", m.Id, m.Cancel))
	script.Append(fmt.Sprintf("goto ${target-%s}\n", m.Id))
}

type IPXEScript struct {
	Script string
}

func (s *IPXEScript) Append(str string) {
	s.Script += str
}

func (s *IPXEScript) Set(k, v string) {
	s.Append(fmt.Sprintf("set %s %s\n", k, v))
}

func (s *IPXEScript) Label(label string) {
	s.Append(fmt.Sprintf(":%s\n", label))
}

func (s *IPXEScript) Goto(label string) {
	s.Append(fmt.Sprintf("goto %s\n", label))
}

func (s *IPXEScript) Echo(str string) {
	s.Append(fmt.Sprintf("echo %s\n", str))
}
