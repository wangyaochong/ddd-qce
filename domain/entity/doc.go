// Package entity provides base entity types for DDD domain modeling:
//
//   - Entity: identity-based entity with equality comparison
//   - AuditableEntity: adds CreatedAt/UpdatedAt timestamps
//   - SoftDeletableEntity: adds DeletedAt soft-delete support
//   - IDGenerator: UUID-based ID generation
//
// Entities are the fundamental unit of identity in DDD. Use the appropriate
// base type depending on your entity's lifecycle requirements.
package entity
