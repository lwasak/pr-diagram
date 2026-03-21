using PRExample.Audit;
using PRExample.Domain;

namespace PRExample.Orders;

/// <summary>
/// Orchestrates order processing through the payment and notification pipeline.
/// Inherits lifecycle management from <see cref="BaseProcessor"/> and implements
/// <see cref="IOrderProcessor"/>.
/// </summary>
public class OrderProcessor : BaseProcessor, IOrderProcessor
{
    // External types from other assemblies — appear as ghost nodes
    private readonly IPaymentService              _payment;
    private readonly ILogger<OrderProcessor>      _logger;
    private readonly INotificationService         _notifications;

    private readonly OrderValidator               _validator;
    private readonly AuditService                 _audit;
    private readonly Dictionary<Guid, OrderStatus> _statusCache = new();

    public Order?       CurrentOrder   { get; private set; }
    public OrderStatus  CurrentStatus  { get; private set; }
    public int          ProcessedCount { get; private set; }

    public OrderProcessor(
        IPaymentService         payment,
        ILogger<OrderProcessor> logger,
        INotificationService    notifications,
        AuditService            audit)
    {
        _payment       = payment;
        _logger        = logger;
        _notifications = notifications;
        _audit         = audit;
        _validator     = new OrderValidator();
        ProcessorName  = nameof(OrderProcessor);
    }

    public override void Initialize()
    {
        _statusCache.Clear();
        MarkInitialized();
    }

    public async Task<OrderResult> ProcessAsync(Order order)
    {
        CurrentOrder = order;
        CurrentStatus = OrderStatus.Processing;

        if (!_validator.Validate(order, out var reason))
        {
            CurrentStatus = OrderStatus.Failed;
            _audit.Record("validation-failed", order);
            return new OrderResult(order.Id, false, OrderStatus.Failed, reason);
        }

        var success = await _payment.ChargeAsync(order.Id, order.Total.Amount);
        CurrentStatus = success ? OrderStatus.Completed : OrderStatus.Failed;
        _statusCache[order.Id] = CurrentStatus;
        ProcessedCount++;

        _audit.Record(success ? "charged" : "charge-failed", order);
        if (success) await _notifications.SendReceiptAsync(order.Customer.Email, order);

        return new OrderResult(order.Id, success, CurrentStatus, success ? null : "Payment declined");
    }

    public OrderStatus GetStatus(Guid orderId) =>
        _statusCache.TryGetValue(orderId, out var s) ? s : OrderStatus.Pending;

    public void Cancel(Guid orderId)
    {
        _statusCache[orderId] = OrderStatus.Failed;
        _audit.Record("cancelled", CurrentOrder!);
    }

    internal AuditEntry? GetLastAudit() => _audit.GetLatest();
}
