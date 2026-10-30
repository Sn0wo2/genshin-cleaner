using Microsoft.UI.Xaml;

namespace UI;

public partial class App : Application
{
    public App()
    {
        InitializeComponent();
    }

    internal static Window? MainWindow { get; private set; }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new MainWindow();
        MainWindow.Activate();
    }
}
