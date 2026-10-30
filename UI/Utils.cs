using System.Globalization;
using System.Text.Json;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace UI;

internal static class Utils
{
    internal static CultureInfo GetCulture(string locale)
    {
        try
        {
            return CultureInfo.GetCultureInfo(locale);
        }
        catch (CultureNotFoundException)
        {
            return CultureInfo.InvariantCulture;
        }
    }

    internal static T? Deserialize<T>(JsonElement data, JsonSerializerOptions options, string? property = null)
        where T : class
    {
        if (property is not null &&
            (data.ValueKind != JsonValueKind.Object || !data.TryGetProperty(property, out data)))
            return null;
        if (data.ValueKind is JsonValueKind.Undefined or JsonValueKind.Null) return null;

        try
        {
            return data.Deserialize<T>(options);
        }
        catch (JsonException)
        {
            return null;
        }
    }

    internal static string FormatBytes(long bytes)
    {
        var units = new[] { "B", "KB", "MB", "GB", "TB" };
        var unit = 0;
        var value = (double)bytes;
        while (value >= 1024 && unit < units.Length - 1)
        {
            value /= 1024;
            unit++;
        }

        return $"{value:0.#} {units[unit]}";
    }

    internal static string FormatFailures<T>(IReadOnlyCollection<T> failures, Func<T, string> format)
    {
        if (failures.Count == 0) return "";
        var lines = failures.Take(5).Select(format).ToList();
        if (failures.Count > 5) lines.Add("…");
        return "\n" + string.Join("\n", lines);
    }

    internal static Task ShowInfo(XamlRoot root, string title, string message)
    {
        return new ContentDialog
            {
                Title = title,
                Content = message,
                CloseButtonText = L.Text("dialog.ok"),
                XamlRoot = root
            }
            .ShowAsync().AsTask();
    }
}
