// A small WPF app showing the Superroute UI kit's components against a configured client (see ExampleSetup.cs).
using System;
using System.Windows;
using System.Windows.Controls;
using Superroute.SApp.UI;
using Superroute.SApp.Wpf;

namespace SAppExample
{
    public static class Program
    {
        [STAThread]
        public static void Main()
        {
            var app = new Application();
            app.Run(new MainWindow());
        }
    }

    public sealed class MainWindow : Window
    {
        private readonly ContentControl _content = new();
        private readonly ListBox _log = new() { Height = 120 };

        public MainWindow()
        {
            Title = "sApp WPF example";
            Width = 1100;
            Height = 800;
            var setup = new ExampleSetup(Show, line => _log.Items.Insert(0, $"{DateTime.Now:T}  {line}"));
            var menu = new ListBox { Width = 220 };
            var catalogue = setup.Catalogue();
            foreach (var (name, _) in catalogue) menu.Items.Add(name);
            menu.SelectionChanged += (_, _) =>
            {
                if (menu.SelectedIndex < 0) return;
                var (name, create) = catalogue[menu.SelectedIndex];
                Show(name, create());
            };
            var grid = new DockPanel();
            DockPanel.SetDock(menu, Dock.Left);
            DockPanel.SetDock(_log, Dock.Bottom);
            grid.Children.Add(menu);
            grid.Children.Add(_log);
            grid.Children.Add(_content);
            Content = grid;
            Resources.MergedDictionaries.Add(SuperrouteTheme.Resources(setup.Context.Theme));
        }

        /// <summary>Shows a component: its WPF control is the class named after the view model.</summary>
        private void Show(string name, ComponentViewModel viewModel)
        {
            var type = typeof(SuperrouteControl).Assembly.GetType($"Superroute.SApp.Wpf.{name}") ?? throw new InvalidOperationException(name);
            if (_content.Content is SuperrouteControl old) old.Model.Dispose();
            _content.Content = (SuperrouteControl)Activator.CreateInstance(type, viewModel)!;
        }
    }
}
