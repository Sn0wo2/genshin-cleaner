using Microsoft.UI.Xaml;

namespace UI;

public partial class App : Application
{
    private Window? _window;

    public App()
    {
        InitializeComponent();
    }

    internal static Window? MainWindow { get; private set; }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        _window = new MainWindow();
        _window.Activate();
        MainWindow = _window;
    }
}