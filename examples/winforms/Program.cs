// A small Windows Forms app showing the Superroute UI kit's components against a configured client (see ../wpf/ExampleSetup.cs).
using System;
using System.Drawing;
using System.Windows.Forms;
using Superroute.SApp.UI;
using Superroute.SApp.WinForms;

namespace SAppExample
{
    public static class Program
    {
        [STAThread]
        public static void Main()
        {
            ApplicationConfiguration.Initialize();
            Application.Run(new MainForm());
        }
    }

    public sealed class MainForm : Form
    {
        private readonly Panel _content = new() { Dock = DockStyle.Fill };
        private readonly ListBox _log = new() { Dock = DockStyle.Bottom, Height = 120 };

        public MainForm()
        {
            Text = "sApp WinForms example";
            ClientSize = new Size(1100, 800);
            var setup = new ExampleSetup(Show, line => _log.Items.Insert(0, $"{DateTime.Now:T}  {line}"));
            var menu = new ListBox { Dock = DockStyle.Left, Width = 220 };
            var catalogue = setup.Catalogue();
            foreach (var (name, _) in catalogue) menu.Items.Add(name);
            menu.SelectedIndexChanged += (_, _) =>
            {
                if (menu.SelectedIndex < 0) return;
                var (name, create) = catalogue[menu.SelectedIndex];
                Show(name, create());
            };
            Controls.Add(_content);
            Controls.Add(menu);
            Controls.Add(_log);
        }

        /// <summary>Shows a component: its WinForms control is the class named after the view model.</summary>
        private void Show(string name, ComponentViewModel viewModel)
        {
            var type = typeof(SuperrouteControl).Assembly.GetType($"Superroute.SApp.WinForms.{name}") ?? throw new InvalidOperationException(name);
            foreach (Control old in _content.Controls)
            {
                if (old is SuperrouteControl component) component.Model.Dispose();
                old.Dispose();
            }
            _content.Controls.Clear();
            var control = (SuperrouteControl)Activator.CreateInstance(type, viewModel)!;
            control.Dock = DockStyle.Fill;
            _content.Controls.Add(control);
        }
    }
}
