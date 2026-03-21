namespace PRExample.Domain;

/// <summary>Identifies the pricing tier for a customer account.</summary>
public enum CustomerTier { Standard, Silver, Gold, Platinum }

/// <summary>Immutable customer profile.</summary>
public record Customer(
    Guid Id,
    string Name,
    string Email,
    CustomerTier Tier
);
