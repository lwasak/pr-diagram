using PRExample.Domain;

namespace PRExample.Audit;

/// <summary>Immutable audit log entry.</summary>
public record AuditEntry(
    Guid      OrderId,
    string    Action,
    AuditLevel Level,
    DateTime  Timestamp
);
