namespace PRExample.Audit;

/// <summary>Query parameters for filtering audit log entries.</summary>
public readonly struct AuditFilter
{
    public DateTime?   From     { get; init; }
    public DateTime?   To       { get; init; }
    public AuditLevel  MinLevel { get; init; }
    public string?     Action   { get; init; }
}
