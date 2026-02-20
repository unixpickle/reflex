package reflex

import "testing"

func TestBuiltInList(t *testing.T) {
	code := `
		List = "stdlib/collections".import.List
		l = List(len=7 value=2)
			.set(idx=2 value=3)!
			.set(idx=5 value=8)!
			.set(idx=6 value=7)!
		result = l.map(fn={result=x.str + " "})!.sum!
	`
	testInterpreterOutput(t, code, "2 2 3 2 2 8 7 ")

	code = `
		List = "stdlib/collections".import.List
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

	code = `
		List = "stdlib/collections".import.List
		l1 = List(len=3 value=1)
			.set(idx=1 value=2)!
			.set(idx=2 value=3)!
		l2 = List(len=4 value=7)
			.set(idx=0 value=-1)!
			.set(idx=3 value=-3)!
		l = l1 + l2
		result =
			l.at(idx=0)!.str + " " +
			l.at(idx=1)!.str + " " +
			l.at(idx=2)!.str + " " +
			l.at(idx=3)!.str + " " +
			l.at(idx=4)!.str + " " +
			l.at(idx=5)!.str + " " +
			l.at(idx=6)!.str + " "
	`
	testInterpreterOutput(t, code, "1 2 3 -1 7 7 -3 ")

	code = `
		List = "stdlib/collections".import.List
		result = List.range(start=3 end=13)!.filter(fn={
			result = x % 2
		})!.map(fn={
			result = x.str + ","
		})!.sum!.substr(end=-1)!
	`
	testInterpreterOutput(t, code, "3,5,7,9,11")

	code = `
		List = "stdlib/collections".import.List
		result = List.range(start=5 end=-3 stride=-2)!.map(fn={
			result = x.str + ","
		})!.sum!.substr(end=-1)!
	`
	testInterpreterOutput(t, code, "5,3,1,-1")

	code = `
		List = "stdlib/collections".import.List
		result = List.range(start=5 end=11 stride=2)!.map(fn={
			result = x.str + ","
		})!.sum!.substr(end=-1)!
	`
	testInterpreterOutput(t, code, "5,7,9")
}
