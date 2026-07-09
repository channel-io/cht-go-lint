package consumer

import "example.com/sib/pkg/sqlrepo" // WANT-VIOLATION: kafka must not import sibling feature sqlrepo

var _ = sqlrepo.DB{}
