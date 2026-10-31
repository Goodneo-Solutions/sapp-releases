// Shared by the WPF and WinForms examples: the client from the environment, the browser sign-in over a loopback
// redirect, and every component with its events wired to a log line or to the next component.
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Net;
using System.Text;
using System.Text.Json.Nodes;
using System.Threading;
using System.Threading.Tasks;
using Superroute.SApp;
using Superroute.SApp.UI;

namespace SAppExample
{
    public sealed class ExampleSetup
    {
        private readonly Action<string, ComponentViewModel> _show;
        private readonly Action<string> _log;

        /// <param name="show">Shows a component (title, view model) in the window.</param>
        /// <param name="log">Writes one line to the window's log.</param>
        public ExampleSetup(Action<string, ComponentViewModel> show, Action<string> log)
        {
            _show = show;
            _log = log;
            // Configured from the environment; nothing here is a credential. Register the redirect URI on the Open Platform console.
            AppCode = Environment.GetEnvironmentVariable("SAPP_APP_CODE") ?? "p_example_app";
            ClientCode = Environment.GetEnvironmentVariable("SAPP_CLIENT_CODE") ?? "k3m9q2";
            RedirectUri = Environment.GetEnvironmentVariable("SAPP_REDIRECT_URI") ?? "http://127.0.0.1:8765/oauth";
            var client = new PartnerClient(
                AppCode,
                clientCode: ClientCode,
                appKey: Environment.GetEnvironmentVariable("SAPP_APP_KEY"),
                tokenStore: OperatingSystem.IsWindows() ? new ProtectedDataTokenStore() : new MemoryTokenStore(),
                baseUrl: Environment.GetEnvironmentVariable("SAPP_BASE_URL") ?? PartnerClient.DefaultBaseUrl,
                locale: Environment.GetEnvironmentVariable("SAPP_LOCALE") ?? "en",
                deviceId: Environment.MachineName);
            Context = new SuperrouteContext(client, new ThemeTokens(new JsonObject { ["primary_color"] = "#1A3D7C", ["display_name"] = "sApp example" }));
            Context.SignInRequired += () => _log("Sign in again.");
        }

        public string AppCode { get; }
        public string ClientCode { get; }
        public string RedirectUri { get; }
        public SuperrouteContext Context { get; }

        /// <summary>Opens the sign-in page in the default browser and waits for it to return to the loopback redirect.</summary>
        public async Task<Uri?> OpenBrowser(Uri url, string redirectUri, CancellationToken ct)
        {
            var redirect = new Uri(redirectUri);
            using var listener = new HttpListener();
            listener.Prefixes.Add($"{redirect.Scheme}://{redirect.Authority}/");
            listener.Start();
            Process.Start(new ProcessStartInfo(url.ToString()) { UseShellExecute = true });
            var contextTask = listener.GetContextAsync();
            var done = await Task.WhenAny(contextTask, Task.Delay(TimeSpan.FromMinutes(5), ct));
            if (done != contextTask) return null;
            var context = await contextTask;
            var page = Encoding.UTF8.GetBytes("<html><body>You can close this window.</body></html>");
            context.Response.ContentType = "text/html";
            await context.Response.OutputStream.WriteAsync(page, ct);
            context.Response.Close();
            return context.Request.Url;
        }

        private static Task<PickedFile?> NoFile(CancellationToken _) => Task.FromResult<PickedFile?>(null);

        /// <summary>Every component, by name, with its events wired.</summary>
        public IReadOnlyList<(string Name, Func<ComponentViewModel> Create)> Catalogue()
        {
            var c = Context;
            return new List<(string, Func<ComponentViewModel>)>
            {
                ("SignIn", () =>
                {
                    var vm = new SignInViewModel(c, new JsonObject { ["authentication"] = new JsonObject { ["methods"] = new JsonObject { ["email"] = true } } }, RedirectUri, OpenBrowser, allowPassword: true);
                    vm.SignedIn += t => _log($"Signed in; scope: {string.Join(" ", t.Scope)}");
                    return vm;
                }),
                ("NotificationInbox", () => { var vm = new NotificationInboxViewModel(c); vm.Open += n => _log($"Open {J.S(n, "deep_link")}"); return vm; }),
                ("WalletSummary", () => { var vm = new WalletSummaryViewModel(c); vm.TopUp += (min, max, cur) => _log($"Top up {min}–{max} {cur}"); return vm; }),
                ("OrderList", () => { var vm = new OrderListViewModel(c); vm.Open += (type, id) => _show("OrderDetail", new OrderDetailViewModel(c, type, id)); return vm; }),
                ("Tracking", () => new TrackingViewModel(c)),
                ("NotificationPreferences", () => new NotificationPreferencesViewModel(c)),
                ("AccountPrivacy", () =>
                {
                    var vm = new AccountPrivacyViewModel(c);
                    vm.OpenLink += url => Process.Start(new ProcessStartInfo(url) { UseShellExecute = true });
                    vm.ClosureRequested += () => _ = c.Client.SignOutAsync();
                    return vm;
                }),
                ("ReturnList", () =>
                {
                    var vm = new ReturnListViewModel(c);
                    vm.Create += () => _show("ReturnForm", new ReturnFormViewModel(c, pickFile: NoFile));
                    vm.Open += id => _log($"Return {id}");
                    return vm;
                }),
                ("ShippingOrderForm", () => { var vm = new ShippingOrderFormViewModel(c); vm.Created += o => _log($"Order {J.S(o, "id")} placed"); return vm; }),
                ("StoreDirectory", () => { var vm = new StoreDirectoryViewModel(c); vm.Open += slug => _show("StorePage", Store(slug)); return vm; }),
                ("GroupBuyList", () => { var vm = new GroupBuyListViewModel(c); vm.Open += code => _show("GroupBuy", GroupBuy(code)); return vm; }),
                ("GroupBuyOrders", () => { var vm = new GroupBuyOrdersViewModel(c); vm.Open += token => _show("GroupBuyOrder", new GroupBuyOrderViewModel(c, token)); return vm; }),
                ("DriverRoutes", () => { var vm = new DriverRoutesViewModel(c); vm.Open += id => _show("RouteStops", Stops(id)); return vm; }),
                ("DriverDuty", () => new DriverDutyViewModel(c)),
                ("DriverPool", () => new DriverPoolViewModel(c)),
                ("RouteBoards", () => { var vm = new RouteBoardsViewModel(c); vm.Open += id => _show("RouteBoard", new RouteBoardViewModel(c, id)); return vm; }),
                ("BoardClaims", () => new BoardClaimsViewModel(c)),
                ("DriverEarnings", () => new DriverEarningsViewModel(c)),
                ("ScanLookup", () => { var vm = new ScanLookupViewModel(c); vm.Found += o => _log($"Found order {J.S(o, "id")}"); return vm; }),
                ("GroupBuyLeading", () =>
                {
                    var vm = new GroupBuyLeadingViewModel(c);
                    vm.Open += id => _show("GroupBuyLeadingGroup", new GroupBuyLeadingGroupViewModel(c, id));
                    vm.OpenPlan += code => _show("GroupBuyLead", new GroupBuyLeadViewModel(c, code));
                    return vm;
                }),
            };
        }

        private StorePageViewModel Store(string slug)
        {
            var vm = new StorePageViewModel(Context, slug);
            vm.OpenProduct += (s, id) =>
            {
                var product = new ProductDetailViewModel(Context, s, id);
                product.Added += cart => _log($"Cart: {J.S(cart, "count")} items");
                product.OpenGroupBuy += code => _show("GroupBuy", GroupBuy(code));
                _show("ProductDetail", product);
            };
            vm.OpenCart += s =>
            {
                var cart = new CartViewModel(Context, s);
                cart.Checkout += store =>
                {
                    var checkout = new CheckoutViewModel(Context, store);
                    // The card sheet is the app's: an app presents it with client_secret / publishable_key.
                    checkout.Payment += (payment, order) => _log($"Card payment for {J.S(order, "code")}");
                    checkout.Placed += order => _log($"Order {J.S(order, "code")} placed");
                    _show("Checkout", checkout);
                };
                _show("Cart", cart);
            };
            return vm;
        }

        private GroupBuyViewModel GroupBuy(string code)
        {
            var vm = new GroupBuyViewModel(Context, code);
            vm.Payment += (payment, order) => _log($"Card hold for {J.S(order, "token")}");
            return vm;
        }

        private RouteStopsViewModel Stops(long routeId)
        {
            var vm = new RouteStopsViewModel(Context, routeId);
            vm.OpenStop += (route, order) =>
            {
                // The position, the camera and the scanner are the app's; this example has none of them.
                var stop = new StopViewModel(Context, route, order, _ => Task.FromResult<GeoPosition?>(null), NoFile);
                stop.Completed += (id, status) => _log($"Stop {id}: {status}");
                stop.CallPhone += phone => _log($"Call {phone}");
                stop.Navigate += (lat, lng, address) => _log($"Navigate to {address}");
                _show("Stop", stop);
            };
            return vm;
        }
    }
}
