// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import "path"

// IsKeyNotFound returns true if the error code is ErrorCodeKeyNotFound.
func IsKeyNotFound(err error) bool {
	if cErr, ok := err.(Error); ok {
		return cErr.Code == ErrorCodeKeyNotFound
	}
	return false
}

// CanonicalURLPath returns the canonical url path for p, which follows the rules:
// 1. the path always starts with "/"
// 2. replace multiple slashes with a single slash
// 3. replace each '.' '..' path name element with equivalent one
// 4. keep the trailing slash
// The function is borrowed from stdlib http.cleanPath in server.go.
func CanonicalURLPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root,
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}
