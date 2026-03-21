#:project Domain/PRExample.Domain.csproj
#:project Audit/PRExample.Audit.csproj
#:project Orders/PRExample.Orders.csproj

using PRExample.Audit;
using PRExample.Domain;
using PRExample.Orders;

// ── Bootstrap ─────────────────────────────────────────────────────────────────
var audit         = new AuditService();
var payment       = new ConsolePaymentService();
var notifications = new ConsoleNotificationService();

var processor = new OrderProcessor(payment, new ConsoleLogger<OrderProcessor>(), notifications, audit);
processor.Initialize();

// ── Run sample orders ─────────────────────────────────────────────────────────
var customer = new Customer(Guid.NewGuid(), "Alice Nguyen", "alice@example.com", CustomerTier.Gold);

var orders = new[]
{
    new Order(Guid.NewGuid(), customer, new Money(249.99m, "USD"), OrderStatus.Pending),
    new Order(Guid.NewGuid(), customer, new Money(0m,      "USD"), OrderStatus.Pending), // fails validation
    new Order(Guid.NewGuid(), customer, new Money(89.50m,  "USD"), OrderStatus.Pending),
};

foreach (var order in orders)
{
    var result = await processor.ProcessAsync(order);
    Console.WriteLine($"[{(result.Success ? "OK" : "FAIL")}] {order.Total}  →  {result.FinalStatus}  {result.FailureReason ?? ""}");
}

Console.WriteLine($"\nProcessed: {processor.ProcessedCount}  |  Audit entries: {audit.EntryCount}");

// ── Minimal in-process stubs (no external packages needed) ───────────────────

sealed class ConsolePaymentService : IPaymentService
{
    public Task<bool> ChargeAsync(Guid orderId, decimal amount)
    {
        var ok = amount > 0;
        Console.WriteLine($"  payment  {orderId:D} ${amount:F2}  → {(ok ? "charged" : "declined")}");
        return Task.FromResult(ok);
    }
}

sealed class ConsoleNotificationService : INotificationService
{
    public Task SendReceiptAsync(string email, Order order)
    {
        Console.WriteLine($"  receipt  → {email}  (${order.Total.Amount:F2})");
        return Task.CompletedTask;
    }
}

sealed class ConsoleLogger<T> : ILogger<T>
{
    public void Log(string message) => Console.WriteLine($"  log      [{typeof(T).Name}] {message}");
}
