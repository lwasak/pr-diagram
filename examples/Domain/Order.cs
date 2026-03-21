namespace PRExample.Domain;

/// <summary>Immutable representation of a customer order.</summary>
public record Order(
    Guid         Id,
    Customer     Customer,
    Money        Total,
    OrderStatus  Status
);
