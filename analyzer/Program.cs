using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

if (args.Length == 0)
{
    Console.Error.WriteLine("Usage: analyzer <file1.cs> [file2.cs ...]");
    return 1;
}

// ── Primitives and well-known framework types to skip as ghost nodes ──────────
var skipTypes = new HashSet<string>(StringComparer.Ordinal)
{
    "bool", "byte", "sbyte", "char", "decimal", "double", "float",
    "int", "uint", "long", "ulong", "short", "ushort", "nint", "nuint",
    "object", "string", "void", "dynamic", "var",
    "Guid", "DateTime", "DateTimeOffset", "TimeSpan", "DateOnly", "TimeOnly",
    "Task", "ValueTask", "CancellationToken",
    "Exception", "ArgumentException", "InvalidOperationException",
    "IDisposable", "IAsyncDisposable",
    "IEnumerable", "ICollection", "IList", "IReadOnlyList", "IReadOnlyCollection",
    "IDictionary", "IReadOnlyDictionary",
    "List", "Dictionary", "HashSet", "Queue", "Stack", "LinkedList",
    "Array", "Span", "Memory", "ReadOnlySpan", "ReadOnlyMemory",
    "Func", "Action", "Predicate", "Comparison", "EventHandler",
    "Type", "Assembly", "Stream", "TextWriter", "TextReader",
    "HttpClient", "Uri", "CultureInfo", "Encoding",
    "JsonElement", "JsonDocument",
};

// ── Parse all source files (syntactic only — no compilation needed) ───────────
var syntaxTrees = new List<SyntaxTree>();
foreach (var path in args)
{
    if (!File.Exists(path))
    {
        Console.Error.WriteLine($"File not found: {path}");
        return 1;
    }
    var src = await File.ReadAllTextAsync(path);
    syntaxTrees.Add(CSharpSyntaxTree.ParseText(src, path: path));
}

// ── Collect all type names defined in the input files ─────────────────────────
var definedNames = new HashSet<string>(StringComparer.Ordinal);
foreach (var tree in syntaxTrees)
{
    var root = await tree.GetRootAsync();
    foreach (var node in root.DescendantNodes())
    {
        var name = node switch
        {
            ClassDeclarationSyntax c     => c.Identifier.Text,
            InterfaceDeclarationSyntax i => i.Identifier.Text,
            RecordDeclarationSyntax r    => r.Identifier.Text,
            EnumDeclarationSyntax e      => e.Identifier.Text,
            StructDeclarationSyntax s    => s.Identifier.Text,
            _                            => null
        };
        if (name != null) definedNames.Add(name);
    }
}

// ── Extract type information ──────────────────────────────────────────────────
var types = new List<TypeDto>();
var referencedExternal = new HashSet<string>(StringComparer.Ordinal);

foreach (var tree in syntaxTrees)
{
    var root = await tree.GetRootAsync();

    foreach (var node in root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>())
    {
        // Skip nested types
        if (node.Parent is BaseTypeDeclarationSyntax) continue;

        var dto = node switch
        {
            ClassDeclarationSyntax c     => ExtractClass(c, referencedExternal),
            InterfaceDeclarationSyntax i => ExtractInterface(i, referencedExternal),
            RecordDeclarationSyntax r    => ExtractRecord(r, referencedExternal),
            EnumDeclarationSyntax e      => ExtractEnum(e),
            StructDeclarationSyntax s    => ExtractStruct(s, referencedExternal),
            _                            => null
        };
        if (dto != null)
        {
            dto.SourceFile = tree.FilePath;
            types.Add(dto);
        }
    }
}

// ── Emit external ghost nodes ─────────────────────────────────────────────────
foreach (var ext in referencedExternal.OrderBy(x => x))
{
    if (!definedNames.Contains(ext) && !skipTypes.Contains(ext))
    {
        types.Add(new TypeDto
        {
            Name = ext,
            Kind = "class",
            IsExternal = true,
        });
    }
}

Console.WriteLine(JsonSerializer.Serialize(types, AnalyzerJsonContext.Default.ListTypeDto));
return 0;

// ── Extraction helpers ────────────────────────────────────────────────────────

static TypeDto ExtractClass(ClassDeclarationSyntax node, HashSet<string> globalReferenced)
{
    var typeParams = TypeParamNames(node.TypeParameterList);
    var local = new HashSet<string>(StringComparer.Ordinal);
    var (baseType, interfaces) = ExtractBases(node.BaseList, local);
    ScanAllReferences(node.Members, local);
    var props   = ExtractProperties(node.Members, local);
    var methods = ExtractMethods(node.Members, local);
    foreach (var tp in typeParams) local.Remove(tp);
    foreach (var r in local) globalReferenced.Add(r);
    return new TypeDto
    {
        Name = node.Identifier.Text,
        Kind = "class",
        IsAbstract = node.Modifiers.Any(SyntaxKind.AbstractKeyword),
        IsInternal = node.Modifiers.Any(SyntaxKind.InternalKeyword) &&
                     !node.Modifiers.Any(SyntaxKind.PublicKeyword),
        Visibility = GetVisibility(node.Modifiers),
        BaseType = baseType,
        Interfaces = interfaces,
        Properties = props,
        Methods = methods,
        Dependencies = local.ToList(),
        TypeParameters = typeParams,
    };
}

static TypeDto ExtractInterface(InterfaceDeclarationSyntax node, HashSet<string> globalReferenced)
{
    var typeParams = TypeParamNames(node.TypeParameterList);
    var local = new HashSet<string>(StringComparer.Ordinal);
    var (_, interfaces) = ExtractBases(node.BaseList, local);
    var props   = ExtractProperties(node.Members, local, isInterface: true);
    var methods = ExtractMethods(node.Members, local, isInterface: true);
    foreach (var tp in typeParams) local.Remove(tp);
    foreach (var r in local) globalReferenced.Add(r);
    return new TypeDto
    {
        Name = node.Identifier.Text,
        Kind = "interface",
        IsInternal = node.Modifiers.Any(SyntaxKind.InternalKeyword) &&
                     !node.Modifiers.Any(SyntaxKind.PublicKeyword),
        Visibility = GetVisibility(node.Modifiers),
        Interfaces = interfaces,
        Properties = props,
        Methods = methods,
        Dependencies = local.ToList(),
        TypeParameters = typeParams,
    };
}

static TypeDto ExtractRecord(RecordDeclarationSyntax node, HashSet<string> globalReferenced)
{
    var typeParams = TypeParamNames(node.TypeParameterList);
    var local = new HashSet<string>(StringComparer.Ordinal);
    ScanAllReferences(node.Members, local);
    var (baseType, interfaces) = ExtractBases(node.BaseList, local);

    var positionalProps = new List<PropertyDto>();
    if (node.ParameterList != null)
    {
        foreach (var param in node.ParameterList.Parameters)
        {
            var typeName = param.Type?.ToString() ?? "object";
            TrackReference(typeName, local);
            positionalProps.Add(new PropertyDto
            {
                Name = param.Identifier.Text,
                Type = typeName,
                Visibility = "public",
            });
        }
    }

    var memberProps = ExtractProperties(node.Members, local);
    var methods     = ExtractMethods(node.Members, local);
    foreach (var tp in typeParams) local.Remove(tp);
    foreach (var r in local) globalReferenced.Add(r);
    return new TypeDto
    {
        Name = node.Identifier.Text,
        Kind = "record",
        Visibility = GetVisibility(node.Modifiers),
        BaseType = baseType,
        Interfaces = interfaces,
        Properties = [.. positionalProps, .. memberProps],
        Methods = methods,
        Dependencies = local.ToList(),
        TypeParameters = typeParams,
    };
}

static TypeDto ExtractEnum(EnumDeclarationSyntax node)
{
    var memberNames = new List<string>();
    var memberValues = new List<string>();
    int nextValue = 0;
    foreach (var member in node.Members)
    {
        if (member.EqualsValue?.Value is LiteralExpressionSyntax lit &&
            int.TryParse(lit.Token.ValueText, out var v))
        {
            nextValue = v;
        }
        memberNames.Add(member.Identifier.Text);
        memberValues.Add(nextValue.ToString());
        nextValue++;
    }
    return new TypeDto
    {
        Name = node.Identifier.Text,
        Kind = "enum",
        IsInternal = node.Modifiers.Any(SyntaxKind.InternalKeyword) &&
                     !node.Modifiers.Any(SyntaxKind.PublicKeyword),
        Visibility = GetVisibility(node.Modifiers),
        Members = memberNames,
        MemberValues = memberValues,
    };
}

static TypeDto ExtractStruct(StructDeclarationSyntax node, HashSet<string> globalReferenced)
{
    var typeParams = TypeParamNames(node.TypeParameterList);
    var local = new HashSet<string>(StringComparer.Ordinal);
    ScanAllReferences(node.Members, local);
    var (_, interfaces) = ExtractBases(node.BaseList, local);
    var props   = ExtractProperties(node.Members, local);
    var methods = ExtractMethods(node.Members, local);
    foreach (var tp in typeParams) local.Remove(tp);
    foreach (var r in local) globalReferenced.Add(r);
    return new TypeDto
    {
        Name = node.Identifier.Text,
        Kind = "struct",
        Visibility = GetVisibility(node.Modifiers),
        Interfaces = interfaces,
        Properties = props,
        Methods = methods,
        Dependencies = local.ToList(),
        TypeParameters = typeParams,
    };
}

static (string? baseType, List<string> interfaces) ExtractBases(
    BaseListSyntax? baseList, HashSet<string> referenced)
{
    if (baseList == null) return (null, []);

    string? baseType = null;
    var interfaces = new List<string>();

    foreach (var type in baseList.Types)
    {
        var name = RootTypeName(type.Type.ToString());
        TrackReference(type.Type.ToString(), referenced);

        if (name.Length > 1 && name[0] == 'I' && char.IsUpper(name[1]))
            interfaces.Add(name);
        else if (baseType == null)
            baseType = name;
        else
            interfaces.Add(name);
    }

    return (baseType, interfaces);
}

static List<PropertyDto> ExtractProperties(
    SyntaxList<MemberDeclarationSyntax> members, HashSet<string> referenced,
    bool isInterface = false)
{
    var result = new List<PropertyDto>();
    foreach (var member in members.OfType<PropertyDeclarationSyntax>())
    {
        var vis = isInterface ? "public" : GetVisibility(member.Modifiers);
        if (vis is not ("public" or "protected" or "internal")) continue;

        var typeName = member.Type.ToString();
        TrackReference(typeName, referenced);
        result.Add(new PropertyDto
        {
            Name = member.Identifier.Text,
            Type = typeName,
            Visibility = vis,
        });
    }
    return result;
}

static List<MethodDto> ExtractMethods(
    SyntaxList<MemberDeclarationSyntax> members, HashSet<string> referenced,
    bool isInterface = false)
{
    var result = new List<MethodDto>();
    foreach (var member in members.OfType<MethodDeclarationSyntax>())
    {
        var vis = isInterface ? "public" : GetVisibility(member.Modifiers);
        if (vis is not ("public" or "protected" or "internal")) continue;

        var returnType = member.ReturnType.ToString();
        TrackReference(returnType, referenced);

        var parameters = member.ParameterList.Parameters.Select(p =>
        {
            var pType = p.Type?.ToString() ?? "object";
            TrackReference(pType, referenced);
            return new ParameterDto { Name = p.Identifier.Text, Type = pType };
        }).ToList();

        result.Add(new MethodDto
        {
            Name = member.Identifier.Text,
            ReturnType = returnType,
            Visibility = vis,
            Parameters = parameters,
        });
    }
    return result;
}

// Scan ALL members (including private fields + constructors) for type references.
// This ensures external types used only in private fields still become ghost nodes.
static void ScanAllReferences(SyntaxList<MemberDeclarationSyntax> members, HashSet<string> referenced)
{
    foreach (var member in members)
    {
        switch (member)
        {
            case FieldDeclarationSyntax field:
                TrackReference(field.Declaration.Type.ToString(), referenced);
                break;
            case ConstructorDeclarationSyntax ctor:
                foreach (var p in ctor.ParameterList.Parameters)
                    TrackReference(p.Type?.ToString() ?? "", referenced);
                break;
        }
    }
}

static List<string> TypeParamNames(TypeParameterListSyntax? list) =>
    list?.Parameters.Select(p => p.Identifier.Text).ToList() ?? [];

static string GetVisibility(SyntaxTokenList modifiers)
{
    if (modifiers.Any(SyntaxKind.PublicKeyword)) return "public";
    if (modifiers.Any(SyntaxKind.ProtectedKeyword)) return "protected";
    if (modifiers.Any(SyntaxKind.InternalKeyword)) return "internal";
    return "private";
}

static string RootTypeName(string type)
{
    type = type.Trim();
    if (type.EndsWith('?')) type = type[..^1];
    var idx = type.IndexOf('<');
    return idx >= 0 ? type[..idx] : type;
}

static void TrackReference(string type, HashSet<string> referenced)
{
    type = type.Trim();
    if (type.EndsWith('?')) type = type[..^1];

    var root = RootTypeName(type);
    if (root.Length > 0) referenced.Add(root);

    var start = type.IndexOf('<');
    if (start >= 0 && type.EndsWith('>'))
    {
        var inner = type[(start + 1)..^1];
        foreach (var part in SplitGenericArgs(inner))
            TrackReference(part.Trim(), referenced);
    }
}

static IEnumerable<string> SplitGenericArgs(string args)
{
    var depth = 0;
    var start = 0;
    for (var i = 0; i < args.Length; i++)
    {
        if (args[i] == '<') depth++;
        else if (args[i] == '>') depth--;
        else if (args[i] == ',' && depth == 0)
        {
            yield return args[start..i];
            start = i + 1;
        }
    }
    if (start < args.Length) yield return args[start..];
}

// ── DTO types ─────────────────────────────────────────────────────────────────

class TypeDto
{
    public string Name { get; set; } = "";
    public string Kind { get; set; } = "class";
    public bool IsExternal { get; set; }
    public bool IsAbstract { get; set; }
    public bool IsInternal { get; set; }
    public string Visibility { get; set; } = "public";
    public string? BaseType { get; set; }
    public List<string> Interfaces { get; set; } = [];
    public List<PropertyDto> Properties { get; set; } = [];
    public List<MethodDto> Methods { get; set; } = [];
    public List<string> Members { get; set; } = [];
    /// <summary>All types referenced by this type (public members + private fields + ctors).</summary>
    public List<string> Dependencies { get; set; } = [];
    /// <summary>Numeric values for enum members (parallel to Members list).</summary>
    public List<string> MemberValues { get; set; } = [];
    /// <summary>Absolute path of the source file this type was parsed from.</summary>
    public string SourceFile { get; set; } = "";
    /// <summary>Generic type parameter names declared on this type (e.g. ["T", "TValue"]).</summary>
    public List<string> TypeParameters { get; set; } = [];
}

class PropertyDto
{
    public string Name { get; set; } = "";
    public string Type { get; set; } = "";
    public string Visibility { get; set; } = "public";
}

class MethodDto
{
    public string Name { get; set; } = "";
    public string ReturnType { get; set; } = "";
    public string Visibility { get; set; } = "public";
    public List<ParameterDto> Parameters { get; set; } = [];
}

class ParameterDto
{
    public string Name { get; set; } = "";
    public string Type { get; set; } = "";
}

// Source-generated JSON context — avoids IL2026 trimming warnings
[JsonSerializable(typeof(List<TypeDto>))]
[JsonSerializable(typeof(TypeDto))]
[JsonSerializable(typeof(List<string>))]
[JsonSerializable(typeof(PropertyDto))]
[JsonSerializable(typeof(MethodDto))]
[JsonSerializable(typeof(ParameterDto))]
[JsonSourceGenerationOptions(
    WriteIndented = true,
    PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase,
    DefaultIgnoreCondition = JsonIgnoreCondition.Never)]
partial class AnalyzerJsonContext : JsonSerializerContext { }
