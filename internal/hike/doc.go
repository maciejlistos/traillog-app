// Package hike implements TrailLog's hike domain.
//
// The package contains the domain models, business services, persistence
// repositories, JSON handlers, and server-rendered web handlers for hikes,
// visits, and tags.
//
// Persistence rules are enforced at multiple levels:
//
//   - services enforce business rules;
//   - repositories own SQL and transaction mechanics;
//   - SQLite constraints protect relational integrity.
//
// HTTP handlers depend on the service layer rather than repositories directly.
package hike
