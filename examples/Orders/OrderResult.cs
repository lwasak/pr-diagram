using PRExample.Domain;

namespace PRExample.Orders;

/// <summary>Outcome of processing a single order.</summary>
public record OrderResult(
    Guid        OrderId,
    bool        Success,
    OrderStatus FinalStatus,
    string?     FailureReason
);
