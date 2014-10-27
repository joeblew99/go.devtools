// iface declares the interface used by test packages.
package iface

type Interface1 interface {
	Method1()
	Method2(a int)
}

type unexportedInterface interface {
	UnexportedInterfaceMethod()
}

type ReturnsValueInterface interface {
	ReturnsSomething(a int) int
}