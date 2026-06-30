package consumer

import "example.com/bad/pkg/sqlrepo" // WANT-VIOLATION: kafka must not import sibling sqlrepo

var _ = sqlrepo.DB{}
