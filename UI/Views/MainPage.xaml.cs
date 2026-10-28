using Windows.Storage.Pickers;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using WinRT.Interop;

namespace UI;

public sealed partial class MainPage : Page
{
    private readonly Backend _backend;
    private readonly string _exePath;
    private readonly CleanerState _state = new();
    private string? _emptyActionKey;

    private string? _emptyMsgKey;

    public MainPage()
    {
        InitializeComponent();
        DataContext = Loc.Instance;

        _exePath = Path.Combine(AppContext.BaseDirectory, "genshin-cleaner-backend.exe");
        _backend = new Backend(_exePath);

        LanguageSelect.ItemsSource = L.Languages;
        LanguageSelect.SelectedValue = L.CurrentLocale;

        L.LanguageChanged += (_, _) =>
        {
            if (ScanStatus.Visibility == Visibility.Visible) ScanStatus.Text = L.Text("scan.noneBody");
            if (_emptyMsgKey is not null) EmptyMsg.Text = L.Text(_emptyMsgKey);
            if (_emptyActionKey is not null) EmptyButton.Content = L.Text(_emptyActionKey);
            UpdateSummary();
        };
    }

    private void OnLanguageChanged(object sender, SelectionChangedEventArgs e)
    {
        if (LanguageSelect.SelectedItem is L.Language language) L.SetLanguage(language.Locale);
    }

    private void OnRulesSelectionChanged(object? sender, EventArgs e)
    {
        UpdateSummary();
    }

    private async void OnScanClick(object sender, RoutedEventArgs e)
    {
        try
        {
            if (_state.Busy) return;
            if (!File.Exists(_exePath))
            {
                await ShowInfo(XamlRoot, L.Text("scan.backendMissingTitle"), L.Text("scan.backendMissingBody"));
                return;
            }

            _state.Selected = null;
            _state.Rules = [];
            Rules.Show(_state);
            UpdateSummary();
            ScanStatus.Visibility = Visibility.Collapsed;
            SetBusy(true, "scan.progress");

            var result = await _backend.List(null, new Progress<Progress>(p => BusyMsg.Text = p.Msg));
            SetBusy(false);

            if (!result.Succeeded)
            {
                if (_state.Installs.Count > 0) ShowEmpty("install.pick", null);
                await ShowInfo(XamlRoot, L.Text("scan.failedTitle"), result.Msg);
                return;
            }

            var installs = ProtocolData.InstallsFrom(result.Data);
            if (installs.Count > 0)
            {
                _state.Installs = installs;
                ShowEmpty("install.pick", null);
                GameSelect.ItemsSource = _state.Installs;
                GameSelect.SelectedIndex = installs.Count == 1 ? 0 : -1;
                return;
            }

            ScanStatus.Text = L.Text("scan.noneBody");
            ScanStatus.Visibility = Visibility.Visible;
            await BrowseAsync();
        }
        catch (Exception ex)
        {
            SetBusy(false);
            await ShowInfo(XamlRoot, L.Text("scan.failedTitle"), ex.Message);
        }
    }

    private void OnBackClick(object sender, RoutedEventArgs e)
    {
        if (_state.Busy) return;
        _state.Selected = null;
        _state.Rules = [];
        Rules.Show(_state);
        UpdateSummary();
        GameSelect.SelectedIndex = -1;
        ShowEmpty("install.pick", null);
        SetState();
    }

    private void OnRefreshClick(object sender, RoutedEventArgs e)
    {
        _ = RefreshPreview();
    }

    private void OnEmptyActionClick(object sender, RoutedEventArgs e)
    {
        _ = RefreshPreview();
    }

    private async void OnActClick(object sender, RoutedEventArgs e)
    {
        try
        {
            if (_state is not { Busy: false, Selected: not null } || !_state.HasSelection) return;

            var action = (string)((Button)sender).Tag;
            var root = _state.Selected;
            var files = _state.SelectedFiles.Select(f => f.Path).ToList();
            var count = files.Count;
            var bytes = ProtocolData.FormatBytes(_state.SelectedBytes);

            var confirm = new ContentDialog
            {
                Title = L.Text(action + ".confirmTitle"),
                Content = new StackPanel
                {
                    Spacing = 8,
                    Children =
                    {
                        new TextBlock
                        {
                            Text = root,
                            FontWeight = FontWeights.Medium,
                            TextTrimming = TextTrimming.CharacterEllipsis
                        },
                        new TextBlock
                        {
                            Text = L.Format(action + ".confirmBody", count, bytes),
                            TextWrapping = TextWrapping.Wrap,
                            Opacity = 0.8
                        }
                    }
                },
                PrimaryButtonText = L.Text(action + ".primary"),
                CloseButtonText = L.Text("dialog.cancel"),
                DefaultButton = ContentDialogButton.Primary,
                XamlRoot = XamlRoot
            };

            if (action == "delete")
            {
                var resources = Application.Current.Resources;
                confirm.Resources["AccentButtonBackground"] = resources["DangerButtonBackground"];
                confirm.Resources["AccentButtonBackgroundPointerOver"] = resources["DangerButtonBackgroundPointerOver"];
                confirm.Resources["AccentButtonBackgroundPressed"] = resources["DangerButtonBackgroundPressed"];
            }

            if (await confirm.ShowAsync() != ContentDialogResult.Primary) return;

            SetBusy(true, action + ".progress");
            var result = await _backend.Act(action, root, files, new Progress<Progress>(p => BusyMsg.Text = p.Msg));
            SetBusy(false);

            if (result.Partial && await ConfirmElevate(action, ProtocolData.DeleteFrom(result.Data)?.Failed.Length ?? 0))
            {
                SetBusy(true, action + ".progress");
                var elevated = await _backend.ElevatedAct(action, root, files);
                SetBusy(false);
                if (elevated is not null) result = elevated;
            }

            var outcome = ProtocolData.DeleteFrom(result.Data);
            if (result.Succeeded)
            {
                var failed = outcome?.Failed ?? [];
                if (result.Partial)
                    await ShowInfo(XamlRoot, L.Text(action + ".partialTitle"),
                        result.Msg + FormatFailed(failed));
                else
                    await ShowInfo(XamlRoot, L.Text(action + ".doneTitle"),
                        L.Format(action + ".doneBody", outcome?.DoneItems ?? 0, count,
                            ProtocolData.FormatBytes(outcome?.FreedBytes ?? 0), bytes));

                var failedPaths = failed.Where(f => f.Path is not null).Select(f => f.Path!)
                    .ToHashSet(StringComparer.OrdinalIgnoreCase);
                _state.Rules = _state.Rules
                    .Select(g => new RuleGroup
                    {
                        Id = g.Id,
                        Params = g.Params,
                        Files = g.Files
                            .Where(f => !files.Contains(f.Path, StringComparer.OrdinalIgnoreCase) ||
                                        failedPaths.Contains(f.Path))
                            .ToList()
                    })
                    .Where(g => g.Files.Count > 0)
                    .ToList();
                Rules.Show(_state);
                if (_state.Rules.Count == 0) ShowEmpty("review.empty", "review.refresh");
                UpdateSummary();
                SetState();
            }
            else
            {
                await ShowInfo(XamlRoot, L.Text(action + ".failedTitle"), result.Msg);
            }
        }
        catch (Exception ex)
        {
            SetBusy(false);
            await ShowInfo(XamlRoot, L.Text("error.title"), ex.Message);
        }
    }

    private async void OnBrowseClick(object sender, RoutedEventArgs e)
    {
        try
        {
            await BrowseAsync();
        }
        catch (Exception ex)
        {
            SetBusy(false);
            await ShowInfo(XamlRoot, L.Text("scan.failedTitle"), ex.Message);
        }
    }

    private async Task BrowseAsync()
    {
        if (_state.Busy) return;
        var picker = new FolderPicker { SuggestedStartLocation = PickerLocationId.ComputerFolder };
        picker.FileTypeFilter.Add("*");
        InitializeWithWindow.Initialize(picker, WindowNative.GetWindowHandle(App.MainWindow!));
        var folder = await picker.PickSingleFolderAsync();
        if (folder is null) return;

        try
        {
            SetBusy(true, "scan.progress");
            var result = await _backend.List(folder.Path, new Progress<Progress>(p => BusyMsg.Text = p.Msg));
            SetBusy(false);

            if (!result.Succeeded)
            {
                await ShowInfo(XamlRoot, L.Text("scan.failedTitle"), result.Msg);
                return;
            }

            var found = ProtocolData.InstallsFrom(result.Data);
            if (found.Count == 0)
            {
                await ShowInfo(XamlRoot, L.Text("scan.failedTitle"),
                    L.Text("scan.noneBody") + FormatFailed(ProtocolData.FailedFrom(result.Data)));
                return;
            }

            foreach (var install in found.Where(install =>
                         !_state.Installs.Any(i => i.Root.Equals(install.Root, StringComparison.OrdinalIgnoreCase))))
                _state.Installs.Add(install);

            GameSelect.ItemsSource = null;
            GameSelect.ItemsSource = _state.Installs;
            GameSelect.SelectedItem =
                _state.Installs.First(i => i.Root.Equals(found[0].Root, StringComparison.OrdinalIgnoreCase));
        }
        catch (Exception ex)
        {
            SetBusy(false);
            await ShowInfo(XamlRoot, L.Text("scan.failedTitle"), ex.Message);
        }
    }

    private async Task<bool> ConfirmElevate(string action, int failedCount)
    {
        var dialog = new ContentDialog
        {
            Title = L.Text(action + ".elevateTitle"),
            Content = L.Format(action + ".elevateBody", failedCount),
            PrimaryButtonText = L.Text(action + ".elevatePrimary"),
            CloseButtonText = L.Text("dialog.cancel"),
            DefaultButton = ContentDialogButton.Primary,
            XamlRoot = XamlRoot
        };
        return await dialog.ShowAsync() == ContentDialogResult.Primary;
    }

    private static string FormatFailed(ProtocolData.Failure[] failed)
    {
        if (failed.Length == 0) return "";
        var lines = failed.Take(5).Select(f => f.Path is null ? f.Reason : f.Path + ": " + f.Reason).ToList();
        if (failed.Length > lines.Count) lines.Add("…");
        return "\n" + string.Join("\n", lines);
    }

    private void OnGameSelected(object sender, SelectionChangedEventArgs e)
    {
        var root = (GameSelect.SelectedItem as InstallInfo)?.Root;
        if (root == _state.Selected) return;
        _state.Selected = root;
        if (root is not null && !_state.Busy) _ = RefreshPreview();
    }

    private async Task RefreshPreview()
    {
        if (_state.Busy || _state.Selected is null) return;

        _state.Rules = [];
        Rules.Show(_state);
        UpdateSummary();
        SetBusy(true, "review.loading");

        var result = await _backend.DryRun(_state.Selected, new Progress<Progress>(p => BusyMsg.Text = p.Msg));

        if (result.Ok)
        {
            _state.Rules = ProtocolData.RuleGroups(result.Data);
            Rules.Show(_state);
            if (_state.Rules.Count == 0) ShowEmpty("review.empty", "review.refresh");
        }
        else
        {
            ShowEmpty("review.dryRunFailedTitle", "review.refresh");
        }

        SetBusy(false);
    }

    private void SetBusy(bool busy, string? msgKey = null)
    {
        _state.Busy = busy;
        if (msgKey is not null) BusyMsg.Text = L.Text(msgKey);
        StartScanButton.IsEnabled = !busy;
        StartBrowseButton.IsEnabled = !busy;
        BrowseButton.IsEnabled = !busy;
        GameSelect.IsEnabled = !busy;
        RescanButton.IsEnabled = !busy;
        RefreshButton.IsEnabled = !busy;
        Rules.IsEnabled = !busy;
        UpdateSummary();
        SetState();
    }

    private void SetState()
    {
        var busy = _state.Busy;
        var hasRules = _state.Rules.Count > 0;
        StartPanel.Visibility = ToVis(!busy && _state.Installs.Count == 0);
        InstallPanel.Visibility = ToVis(_state.Installs.Count > 0);
        BusyPanel.Visibility = ToVis(busy);
        ReviewPanel.Visibility = ToVis(!busy && hasRules);
        BackButton.Visibility = ReviewPanel.Visibility;
        EmptyPanel.Visibility = ToVis(!busy && _state.Installs.Count > 0 && !hasRules);
    }

    private void ShowEmpty(string msgKey, string? actionKey)
    {
        _emptyMsgKey = msgKey;
        _emptyActionKey = actionKey;
        EmptyMsg.Text = L.Text(msgKey);
        EmptyButton.Content = actionKey is null ? null : L.Text(actionKey);
        EmptyButton.Visibility = actionKey is null ? Visibility.Collapsed : Visibility.Visible;
    }

    private void UpdateSummary()
    {
        var total = _state.Rules.Sum(r => r.Files.Count);
        SummaryText.Text = total == 0
            ? ""
            : L.Format("rules.selectionSummary",
                _state.SelectedCount, total,
                ProtocolData.FormatBytes(_state.SelectedBytes),
                ProtocolData.FormatBytes(_state.Rules.Sum(r => r.Files.Sum(f => f.Size))));
        DeleteButton.IsEnabled = StubButton.IsEnabled = UnstubButton.IsEnabled =
            _state is { Busy: false, HasSelection: true };
    }

    private static Visibility ToVis(bool visible)
    {
        return visible ? Visibility.Visible : Visibility.Collapsed;
    }

    private static Task ShowInfo(XamlRoot root, string title, string msg)
    {
        return new ContentDialog
                { Title = title, Content = msg, CloseButtonText = L.Text("dialog.ok"), XamlRoot = root }
            .ShowAsync().AsTask();
    }
}