package model

// Layer direction (from the shared template): model is the bottom layer, so it
// must not import up to svc. AST-only fixture, so the model<->svc cycle is fine.
import "example.com/badsvc/internal/domain/order/svc" // WANT-VIOLATION: model must not import up to svc (layer direction)

var _ = svc.Do
