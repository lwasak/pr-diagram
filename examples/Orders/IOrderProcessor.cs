using PRExample.Domain;

namespace PRExample.Orders;

/// <summary>Processes customer orders through the payment pipeline.</summary>
public interface IOrderProcessor
{
    Task<OrderResult> ProcessAsync(Order order);
    OrderStatus       GetStatus(Guid orderId);
    void              Cancel(Guid orderId);
}
