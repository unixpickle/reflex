package reflex

import "testing"

func TestBuiltInList(t *testing.T) {
	code := `
		List = "stdio/collections".import!.List
		l = List(len=7 value=2)
			.set(idx=2 value=3)!
			.set(idx=5 value=8)!
			.set(idx=6 value=7)!
		result = l.map(fn={result=x.str + " "})!.sum!
	`
	testInterpreterOutput(t, code, "2 2 3 2 2 8 7 ")

	code = `
		List = "stdio/collections".import!.List
		l = List(len=7 value=2)
			.set(idx=2 value=3)!
			.set(idx=5 value=8)!
			.set(idx=6 value=7)!
		result =
			l.at(idx=0)!.str + " " +
			l.at(idx=1)!.str + " " +
			l.at(idx=2)!.str + " " +
			l.at(idx=3)!.str + " " +
			l.at(idx=4)!.str + " " +
			l.at(idx=5)!.str + " " +
			l.at(idx=6)!.str + " "
	`
	testInterpreterOutput(t, code, "2 2 3 2 2 8 7 ")
}
