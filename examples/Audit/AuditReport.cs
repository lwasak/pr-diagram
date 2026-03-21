namespace PRExample.Audit;

/// <summary>Aggregated audit report produced by <see cref="AuditService"/>.</summary>
public record AuditReport(
    IReadOnlyList<AuditEntry> Entries,
    int                       TotalCount,
    AuditLevel                HighestLevel,
    DateTime                  GeneratedAt
);
