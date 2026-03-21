using PRExample.Domain;

namespace PRExample.Orders;

/// <summary>Validates orders before they enter the payment pipeline.</summary>
public class OrderValidator
{
    private readonly decimal _maxOrderAmount;

    public OrderValidator(decimal maxOrderAmount = 100_000m)
    {
        _maxOrderAmount = maxOrderAmount;
    }

    public bool Validate(Order order, out string? reason)
    {
        if (order.Total.Amount <= 0)          { reason = "Amount must be positive";          return false; }
        if (order.Total.Amount > _maxOrderAmount) { reason = "Amount exceeds limit";         return false; }
        if (string.IsNullOrWhiteSpace(order.Customer.Name)) { reason = "Customer name empty"; return false; }
        reason = null;
        return true;
    }
}
