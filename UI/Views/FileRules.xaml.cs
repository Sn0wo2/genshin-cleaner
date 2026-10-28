using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace UI;

public sealed partial class FileRules : UserControl
{
    public FileRules()
    {
        InitializeComponent();
        L.LanguageChanged += (_, _) =>
        {
            if (List.ItemsSource is not List<RuleGroup> groups) return;
            foreach (var g in groups)
            {
                g.RefreshTexts();
                foreach (var f in g.Files) f.RefreshTexts();
            }
        };
    }

    public event EventHandler? SelectionChanged;

    public void Show(CleanerState state)
    {
        List.ItemsSource = state.Rules;
    }

    private void OnFileClick(object sender, RoutedEventArgs e)
    {
        SelectionChanged?.Invoke(this, EventArgs.Empty);
    }
}