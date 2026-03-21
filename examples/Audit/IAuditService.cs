using PRExample.Domain;

namespace PRExample.Audit;

public interface IAuditService
{
    void        Record(string action, Order order);
    AuditEntry? GetLatest();
    IReadOnlyList<AuditEntry> GetAll();
    AuditReport GenerateReport(AuditFilter filter);
}
