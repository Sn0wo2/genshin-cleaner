using System.ComponentModel;
using System.Globalization;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using CliWrap;
using System.Diagnostics;
using Microsoft.UI.Xaml;

namespace UI;

public static class L
{
    private const string FallbackLocale = "en-US";
    private static readonly Dictionary<string, Dictionary<string, string>> _packs = new();
    private static readonly List<Language> _languages = [];
    private static Dictionary<string, string> _current = [];
    private static CultureInfo? _culture;

    static L()
    {
        var dir = Path.Combine(AppContext.BaseDirectory, "Assets", "Locales");
        if (Directory.Exists(dir))
            foreach (var file in Directory.EnumerateFiles(dir, "*.json"))
                try
                {
                    using var doc = JsonDocument.Parse(File.ReadAllText(file));
                    var root = doc.RootElement;
                    if (root.ValueKind != JsonValueKind.Object ||
                        !root.TryGetProperty("locale", out var localeEl) ||
                        localeEl.GetString() is not { } locale ||
                        !_packs.TryAdd(locale, ReadStrings(root)))
                        continue;

                    var name = root.TryGetProperty("name", out var nameEl) &&
                               nameEl.ValueKind == JsonValueKind.String
                        ? nameEl.GetString()!
                        : locale;
                    _languages.Add(new Language(locale, name));
                }
                catch
                {
                }

        CurrentLocale = PickInitial();
        if (_packs.TryGetValue(CurrentLocale, out var pack)) _current = pack;
    }

    public static string CurrentLocale { get; private set; } = FallbackLocale;
    public static IReadOnlyList<Language> Languages { get; private set; } = _languages;

    private static CultureInfo Culture => _culture ??= CreateCulture();

    private static string SettingsPath =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "GenshinCleaner", "settings.json");

    public static event EventHandler? LanguageChanged;

    public static void SetLanguage(string locale)
    {
        if (locale == CurrentLocale || !_packs.TryGetValue(locale, out var pack)) return;

        _current = pack;
        CurrentLocale = locale;
        _culture = null;
        SaveLanguage(locale);
        LanguageChanged?.Invoke(null, EventArgs.Empty);
        Loc.Instance.Refresh();
    }

    public static string Text(string key)
    {
        return TryGet(key) ?? key;
    }

    public static string Format(string key, params object?[] args)
    {
        var text = TryGet(key) ?? key;
        return args.Length == 0 ? text : string.Format(Culture, text, args);
    }

    public static string Rule(string id, string kind, params object?[] args)
    {
        var text = TryGet($"rules.{id}.{kind}");
        if (text is null) return kind == "note" ? "" : id;
        return args.Length == 0 ? text : string.Format(Culture, text, args);
    }

    private static CultureInfo CreateCulture()
    {
        try
        {
            return CultureInfo.GetCultureInfo(CurrentLocale);
        }
        catch (CultureNotFoundException)
        {
            return CultureInfo.InvariantCulture;
        }
    }

    private static string? TryGet(string key)
    {
        if (_current.TryGetValue(key, out var value)) return value;
        return _packs.TryGetValue(FallbackLocale, out var fallback) &&
               fallback.TryGetValue(key, out var text)
            ? text
            : null;
    }

    private static Dictionary<string, string> ReadStrings(JsonElement root)
    {
        var strings = new Dictionary<string, string>();
        if (root.TryGetProperty("strings", out var el) && el.ValueKind == JsonValueKind.Object)
            foreach (var property in el.EnumerateObject())
                if (property.Value.ValueKind == JsonValueKind.String)
                    strings[property.Name] = property.Value.GetString()!;

        return strings;
    }

    private static string PickInitial()
    {
        var saved = ReadSavedLanguage();
        if (saved is not null && _packs.ContainsKey(saved)) return saved;

        var system = CultureInfo.CurrentUICulture.Name;
        if (_packs.ContainsKey(system)) return system;

        var language = system.Split('-')[0];
        var match = _languages.FirstOrDefault(l =>
            l.Locale.StartsWith(language + "-", StringComparison.OrdinalIgnoreCase));
        if (match is not null) return match.Locale;

        return _packs.ContainsKey(FallbackLocale)
            ? FallbackLocale
            : _languages.FirstOrDefault()?.Locale ?? FallbackLocale;
    }

    private static string? ReadSavedLanguage()
    {
        try
        {
            using var doc = JsonDocument.Parse(File.ReadAllText(SettingsPath));
            return doc.RootElement.ValueKind == JsonValueKind.Object &&
                   doc.RootElement.TryGetProperty("language", out var el) &&
                   el.ValueKind == JsonValueKind.String
                ? el.GetString()
                : null;
        }
        catch
        {
            return null;
        }
    }

    private static void SaveLanguage(string locale)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(SettingsPath)!);
            File.WriteAllText(SettingsPath, JsonSerializer.Serialize(new { language = locale }));
        }
        catch
        {
        }
    }

    public sealed record Language(string Locale, string Name);
}

public sealed class Loc : INotifyPropertyChanged
{
    public static Loc Instance { get; } = new();

    public string this[string key] => L.Text(key);

    public event PropertyChangedEventHandler? PropertyChanged;

    internal void Refresh()
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(string.Empty));
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs("Item[]"));
    }
}

public sealed record Progress(string Msg);

public sealed record Result(int Status, string Msg, JsonElement Data)
{
    public bool Ok => Status is 200;
    public bool Partial => Status is 202;
    public bool Succeeded => Ok || Partial;
}

public sealed class Backend(string exePath)
{
    private const int Protocol = 1;

    public Task<Result> List(string? root = null, IProgress<Progress>? progress = null, CancellationToken ct = default)
    {
        return Run("list", root, null, progress, ct);
    }

    public Task<Result> DryRun(string? root, IProgress<Progress>? progress = null, CancellationToken ct = default)
    {
        return Run("dry", root, null, progress, ct);
    }

    public Task<Result> Act(string action, string root, IEnumerable<string> files, IProgress<Progress>? progress = null,
        CancellationToken ct = default)
    {
        return Run(action, root, JsonSerializer.Serialize(new { keep = files }), progress, ct);
    }

    private async Task<Result> Run(string action, string? root, string? selection, IProgress<Progress>? progress,
        CancellationToken ct)
    {
        var args = new List<string> { "--protocol", Protocol.ToString() };
        if (action != "dry") args.Add("--" + action);
        if (selection is not null) args.Add("--stdin");
        if (!string.IsNullOrEmpty(root)) args.Add(root);

        Result? result = null;
        var stderrTail = new List<string>();

        CommandResult exit;
        try
        {
            exit = await Cli.Wrap(exePath)
                .WithArguments(args)
                .WithStandardInputPipe(selection is null
                    ? PipeSource.Null
                    : PipeSource.FromString(selection, Encoding.UTF8))
                .WithStandardOutputPipe(PipeTarget.ToDelegate(line =>
                {
                    var env = Parse(line);
                    if (env is null) return;
                    if (env.Value.Status is >= 200 and < 300)
                        result ??= new Result(env.Value.Status, env.Value.Msg, env.Value.Data);
                    else
                        progress?.Report(new Progress(env.Value.Msg));
                }, Encoding.UTF8))
                .WithStandardErrorPipe(PipeTarget.ToDelegate(line =>
                {
                    stderrTail.Add(line);
                    if (stderrTail.Count > 64) stderrTail.RemoveAt(0);
                }, Encoding.UTF8))
                .WithValidation(CommandResultValidation.None)
                .ExecuteAsync(ct);
        }
        catch (Exception ex)
        {
            // never fail silently: launch/pipe errors (e.g. AV blocking the exe) surface as a failed result
            return new Result(500, "backend launch failed: " + ex.Message, default);
        }

        if (result is not null) return result;

        var tail = string.Join("\n", stderrTail.TakeLast(5));
        return new Result(500, "backend exited without a result (exit code " + exit.ExitCode + ")" +
                               (tail.Length > 0 ? "\n" + tail : ""), default);
    }

    /// <summary>
    ///     Re-runs an action in an elevated backend (UAC prompt on the backend, not the UI). An elevated child
    ///     cannot share our stdin/stdout pipes, so the selection goes in via a temp file and the result comes
    ///     back via stdout redirection into one.
    /// </summary>
    public async Task<Result?> ElevatedAct(string action, string root, IEnumerable<string> files)
    {
        var sel = Path.Combine(Path.GetTempPath(), "genshin-cleaner-" + Guid.NewGuid().ToString("N") + ".json");
        var res = Path.Combine(Path.GetTempPath(), "genshin-cleaner-" + Guid.NewGuid().ToString("N") + ".json");
        await File.WriteAllTextAsync(sel, JsonSerializer.Serialize(new { keep = files }));
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = "cmd.exe",
                Verb = "runas",
                UseShellExecute = true,
                WindowStyle = ProcessWindowStyle.Hidden,
                Arguments =
                    $"/c \"\"{exePath}\" --protocol {Protocol} --{action} --selection-file \"{sel}\" \"{root}\" > \"{res}\" 2>nul\""
            };
            using var process = Process.Start(psi);
            if (process is null) return null;
            await process.WaitForExitAsync();

            foreach (var line in File.Exists(res) ? File.ReadAllLines(res) : [])
            {
                var env = Parse(line);
                if (env is { Status: >= 200 and < 300 } v) return new Result(v.Status, v.Msg, v.Data);
            }

            return null;
        }
        catch (Win32Exception)
        {
            return null; // user dismissed the UAC prompt
        }
        finally
        {
            try { File.Delete(sel); } catch { /* best effort */ }
            try { File.Delete(res); } catch { /* best effort */ }
        }
    }

    private static (int Status, string Msg, JsonElement Data)? Parse(string line)
    {
        JsonElement env;
        try
        {
            using var doc = JsonDocument.Parse(line);
            env = doc.RootElement.Clone();
        }
        catch (JsonException)
        {
            return null;
        }

        if (env.ValueKind != JsonValueKind.Object ||
            !env.TryGetProperty("status", out var st) ||
            st.ValueKind != JsonValueKind.Number ||
            !st.TryGetInt32(out var status) ||
            !env.TryGetProperty("msg", out var msgEl) ||
            msgEl.ValueKind != JsonValueKind.String)
            return null;

        var data = env.TryGetProperty("data", out var dataEl) && dataEl.ValueKind == JsonValueKind.Object
            ? dataEl.Clone()
            : default;

        return (status, msgEl.GetString() ?? "", data);
    }
}

public static class ProtocolData
{
    private static readonly JsonSerializerOptions Json = new()
    {
        PropertyNameCaseInsensitive = true,
        NumberHandling = JsonNumberHandling.AllowReadingFromString
    };

    public static List<InstallInfo> InstallsFrom(JsonElement data)
    {
        return Field<List<InstallDto>>(data, "genshins")?
            .Where(g => g.Root is not null)
            .Select(g => new InstallInfo(g.Root!))
            .ToList() ?? [];
    }

    public static Failure[] FailedFrom(JsonElement data)
    {
        return Field<Failure[]>(data, "failed") ?? [];
    }

    public static List<RuleGroup> RuleGroups(JsonElement data)
    {
        return Field<List<RuleDto>>(data, "rules")?
            .Select(r => new RuleGroup
            {
                Id = r.Id ?? "",
                Params = r.Params ?? [],
                Files = (r.Targets ?? [])
                    .Where(t => t.Path is not null)
                    .Select(t => new FileItem { Path = t.Path!, Size = t.SizeBytes ?? 0 })
                    .ToList()
            })
            .Where(g => g.Files.Count > 0)
            .ToList() ?? [];
    }

    public static DeleteOutcome? DeleteFrom(JsonElement data)
    {
        return data.ValueKind == JsonValueKind.Object ? Try(() => data.Deserialize<DeleteOutcome>(Json)) : null;
    }

    private static T? Field<T>(JsonElement data, string name) where T : class
    {
        return data.ValueKind == JsonValueKind.Object &&
               data.TryGetProperty(name, out var el) &&
               el.ValueKind == JsonValueKind.Array
            ? Try(() => el.Deserialize<T>(Json))
            : null;
    }

    private static T? Try<T>(Func<T?> read) where T : class
    {
        try
        {
            return read();
        }
        catch (JsonException)
        {
            return null;
        }
    }

    public static string FormatBytes(long bytes)
    {
        string[] units = ["B", "KB", "MB", "GB", "TB"];
        var i = 0;
        double d = bytes;
        while (d >= 1024 && i < units.Length - 1)
        {
            d /= 1024;
            i++;
        }

        return $"{d:0.#} {units[i]}";
    }

    private sealed record InstallDto(string? Root);

    private sealed record TargetDto(string? Path, long? SizeBytes);

    private sealed record RuleDto(string? Id, string[]? Params, List<TargetDto>? Targets);

    public sealed record Failure(string? Path, string Reason);

    public sealed record DeleteOutcome(long DoneItems, long FreedBytes, Failure[] Failed);
}

public sealed record InstallInfo(string Root);

public sealed class RuleGroup : INotifyPropertyChanged
{
    public required string Id { get; init; }
    public string[] Params { get; init; } = [];
    public required List<FileItem> Files { get; init; }

    public string Title => L.Rule(Id, "title");
    public string Note => L.Rule(Id, "note", Params);
    public Visibility NoteVisibility => Note.Length > 0 ? Visibility.Visible : Visibility.Collapsed;

    public string Summary => L.Format("rules.groupSummary", Files.Count,
        ProtocolData.FormatBytes(Files.Sum(f => f.Size)));

    public event PropertyChangedEventHandler? PropertyChanged;

    internal void RefreshTexts()
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(string.Empty));
    }
}

public sealed class FileItem : INotifyPropertyChanged
{
    private bool _selected = true;
    public required string Path { get; init; }
    public long Size { get; init; }

    public bool Selected
    {
        get => _selected;
        set
        {
            if (_selected == value) return;
            _selected = value;
            OnPropertyChanged();
        }
    }

    public string SizeText => ProtocolData.FormatBytes(Size);
    public string TooltipText => L.Format("rules.fileTooltip", Path, SizeText, Size.ToString("N0"));

    public event PropertyChangedEventHandler? PropertyChanged;

    internal void RefreshTexts()
    {
        OnPropertyChanged(nameof(TooltipText));
    }

    private void OnPropertyChanged([CallerMemberName] string? name = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(name));
    }
}

public sealed class CleanerState
{
    public bool Busy { get; set; }

    public List<InstallInfo> Installs { get; set; } = [];
    public string? Selected { get; set; }
    public List<RuleGroup> Rules { get; set; } = [];

    public IEnumerable<FileItem> SelectedFiles => Rules.SelectMany(r => r.Files).Where(f => f.Selected);
    public bool HasSelection => SelectedFiles.Any();
    public int SelectedCount => SelectedFiles.Count();
    public long SelectedBytes => SelectedFiles.Sum(f => f.Size);
}