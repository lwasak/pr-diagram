using PRExample.Domain;

namespace PRExample.Audit;

/// <summary>Writes and queries the audit log.</summary>
public class AuditService : IAuditService
{
    private readonly List<AuditEntry> _log = new();

    public int        EntryCount { get; private set; }
    public AuditLevel MinLevel   { get; set; } = AuditLevel.Info;

    public void Record(string action, Order order)
    {
        var level = action.Contains("failed") ? AuditLevel.Warning : AuditLevel.Info;
        if (level < MinLevel) return;
        _log.Add(new AuditEntry(order.Id, action, level, DateTime.UtcNow));
        EntryCount++;
    }

    public AuditEntry? GetLatest() => _log.Count > 0 ? _log[^1] : null;

    public IReadOnlyList<AuditEntry> GetAll() => _log.AsReadOnly();

    public AuditReport GenerateReport(AuditFilter filter)
    {
        var entries = _log
            .Where(e => (filter.From   is null || e.Timestamp >= filter.From) &&
                        (filter.To     is null || e.Timestamp <= filter.To)   &&
                        (filter.Action is null || e.Action == filter.Action)  &&
                        e.Level >= filter.MinLevel)
            .ToList();

        var highest = entries.Count > 0 ? entries.Max(e => e.Level) : AuditLevel.Info;
        return new AuditReport(entries, entries.Count, highest, DateTime.UtcNow);
    }
}
