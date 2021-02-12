package kiterrno

func init() {
	registeredErrors = make(map[int]*ErrorCategory)

	RegisterErrors("UNREGISTERED", UNREGISTERED, make(map[int]string))
	RegisterErrors("THRIFT", THRIFT, thriftErrorCodeDesc)
	RegisterErrors("KITE", KITE, kiteErrorCodeDesc)
	RegisterErrors("MESH", MESH, meshErrorCodeDesc)
}
