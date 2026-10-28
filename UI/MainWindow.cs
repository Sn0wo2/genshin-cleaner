using Windows.Graphics;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;

namespace UI;

public sealed class MainWindow : Window
{
    public MainWindow()
    {
        Title = L.Text("app.title");
        ExtendsContentIntoTitleBar = true;
        AppWindow.SetIcon("Assets/AppIcon.ico");
        SystemBackdrop = new MicaBackdrop();

        var titleBar = new TitleBar
        {
            Title = L.Text("app.title"),
            IconSource = new ImageIconSource { ImageSource = new BitmapImage(new Uri("ms-appx:///Assets/AppIcon.ico")) }
        };
        L.LanguageChanged += (_, _) =>
        {
            Title = L.Text("app.title");
            titleBar.Title = L.Text("app.title");
        };

        var page = new MainPage();

        Grid.SetRow(titleBar, 0);
        Grid.SetRow(page, 1);
        var root = new Grid
        {
            RowDefinitions =
            {
                new RowDefinition { Height = GridLength.Auto },
                new RowDefinition { Height = new GridLength(1, GridUnitType.Star) }
            },
            Children = { titleBar, page }
        };

        Content = root;

        AppWindow.Resize(new SizeInt32(1280, 920));
    }
}
