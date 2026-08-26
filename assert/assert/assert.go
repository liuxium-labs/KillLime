package assert

import "github.com/killlime/killlime/oerror"

func IsTrue(ok bool, message string, args ...interface{}) {
	if !ok {
		panic(oerror.New(message, args...))
	}
}
