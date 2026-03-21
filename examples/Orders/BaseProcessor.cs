namespace PRExample.Orders;

/// <summary>Base class providing common processor lifecycle hooks.</summary>
public abstract class BaseProcessor
{
    protected string ProcessorName  { get; init; } = string.Empty;
    protected bool   IsInitialized  { get; private set; }

    public abstract void Initialize();

    protected void MarkInitialized() => IsInitialized = true;
}
