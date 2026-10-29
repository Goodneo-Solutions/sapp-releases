import Foundation
import SApp
import SAppUI
import SwiftUI

// The example reads its client from the environment; without it the components show their error states.
//   SAPP_APP_CODE=p_acme_parcels SAPP_CLIENT_CODE=k3m9q2 SAPP_LOCALE=fr swift run
let environment = ProcessInfo.processInfo.environment

@MainActor
final class Demo: ObservableObject {
    let client = PartnerClient(
        appCode: environment["SAPP_APP_CODE"] ?? "p_example",
        clientCode: environment["SAPP_CLIENT_CODE"] ?? "example",
        appKey: environment["SAPP_APP_KEY"],
        locale: environment["SAPP_LOCALE"]
    )
    let theme = ThemeTokens(branding: ["primary_color": .string(environment["SAPP_PRIMARY_COLOR"] ?? "#1A3D7C")])
    /// The last event a component raised; the app decides what each one does.
    @Published var event = ""

    func log(_ text: String) { event = text }
}

struct ExampleView: View {
    @StateObject private var demo = Demo()
    @State private var selection: String? = "Sign-in"

    private let names = [
        "Sign-in", "Notification inbox", "Wallet summary", "Order list", "Pickup list",
        "Order detail", "Tracking", "Notification preferences", "Account and privacy", "Locker map", "Return list",
        "Return form", "Shipping order form",
        "Store directory", "Store page", "Product detail", "Cart", "Checkout",
        "Group buy list", "Group buy", "Group buy orders", "Group buy order",
        "Driver routes", "Route stops", "Stop", "Duty and offers", "Pool", "Route boards", "Route board", "Board claims",
        "Earnings", "Scan lookup",
        "Lead a group buy", "Groups I lead", "A group I lead",
    ]

    var body: some View {
        SuperrouteScope(client: demo.client, theme: demo.theme, onSignInRequired: { demo.log("onSignInRequired") }) {
            NavigationView {
                List(names, id: \.self, selection: $selection) { name in
                    NavigationLink(name, tag: name, selection: $selection) { page(name) }
                }
                .frame(minWidth: 220)
                Text(selection ?? "")
            }
        }
        .frame(minWidth: 900, minHeight: 640)
    }

    @ViewBuilder
    private func page(_ name: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            component(name).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top).padding()
            Divider()
            Text(demo.event).font(.caption.monospaced()).padding(8).frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private func component(_ name: String) -> some View {
        let log = demo.log
        switch name {
        case "Sign-in":
            SignIn(bootstrap: ["authentication": ["methods": ["email": true]]], redirectUri: "example:/oauth",
                   openBrowser: { url, _ in log("openBrowser \(url)"); return nil }, allowPassword: true,
                   onSignedIn: { _ in log("onSignedIn") })
        case "Notification inbox": NotificationInbox(onOpen: { log("onOpen \($0["id"]?.srText ?? "")") })
        case "Wallet summary": WalletSummary(onTopUp: { min, max, currency in log("onTopUp \(min ?? "") \(max ?? "") \(currency ?? "")") })
        case "Order list": OrderList(onOpen: { log("onOpen \($0) \($1)") })
        case "Pickup list": PickupList(onShowCode: { log("onShowCode \($0)") })
        case "Order detail": OrderDetail(type: "shipping", id: 1, onTrack: { log("onTrack \($0)") })
        case "Tracking": Tracking()
        case "Notification preferences": NotificationPreferences()
        case "Account and privacy": AccountPrivacy(onOpenLink: { log("onOpenLink \($0)") }, onClosureRequested: { log("onClosureRequested") })
        case "Locker map": LockerMap(onSelect: { log("onSelect \($0["id"]?.srText ?? "")") })
        case "Return list": ReturnList(onOpen: { log("onOpen \($0)") }, onCreate: { log("onCreate") })
        case "Return form": ReturnForm(pickFile: { log("pickFile"); return nil }, onCreated: { _ in log("onCreated") }, onCancel: { log("onCancel") })
        case "Shipping order form": ShippingOrderForm(onCreated: { _ in log("onCreated") }, onCancel: { log("onCancel") })
        case "Store directory": StoreDirectory(onOpen: { log("onOpen \($0)") })
        case "Store page": StorePage(slug: "example", onOpenProduct: { log("onOpenProduct \($0) \($1)") }, onOpenCart: { log("onOpenCart \($0)") })
        case "Product detail": ProductDetail(slug: "example", productId: 1, onAdded: { _ in log("onAdded") }, onOpenGroupBuy: { log("onOpenGroupBuy \($0)") })
        case "Cart": Cart(slug: "example", onCheckout: { log("onCheckout \($0)") })
        case "Checkout": Checkout(slug: "example", onPayment: { _, _ in log("onPayment") }, onPlaced: { _ in log("onPlaced") })
        case "Group buy list": GroupBuyList(onOpen: { log("onOpen \($0)") })
        case "Group buy": GroupBuy(code: "example", onPayment: { _, _ in log("onPayment") })
        case "Group buy orders": GroupBuyOrders(onOpen: { log("onOpen \($0)") })
        case "Group buy order": GroupBuyOrder(token: "example", onPay: { log("onPay \($0)") })
        case "Driver routes": DriverRoutes(onOpen: { log("onOpen \($0)") })
        case "Route stops": RouteStops(routeId: 1, onOpenStop: { log("onOpenStop \($0) \($1)") })
        case "Stop":
            Stop(routeId: 1, orderId: 1, getPosition: { DriverPosition(latitude: 43.65, longitude: -79.38) },
                 pickPhoto: { log("pickPhoto"); return nil }, onCompleted: { log("onCompleted \($0) \($1)") })
        case "Duty and offers": DriverDuty()
        case "Pool": DriverPool()
        case "Route boards": RouteBoards(onOpen: { log("onOpen \($0)") })
        case "Route board": RouteBoard(boardId: 1)
        case "Board claims": BoardClaims()
        case "Earnings": DriverEarnings()
        case "Scan lookup": ScanLookup(scan: { "TRACK-001" }, onFound: { log("onFound \($0["id"]?.srText ?? "")") })
        case "Lead a group buy": GroupBuyLead(code: "example", onShare: { _, url in log("onShare \(url)") }, onStarted: { log("onStarted \($0)") })
        case "Groups I lead": GroupBuyLeading(onOpen: { log("onOpen \($0)") }, onOpenPlan: { log("onOpenPlan \($0)") })
        case "A group I lead": GroupBuyLeadingGroup(groupId: 1, onShare: { _, url in log("onShare \(url)") })
        default: EmptyView()
        }
    }
}

struct ExampleApp: App {
    var body: some Scene {
        WindowGroup("sApp UI kit") { ExampleView() }
    }
}

ExampleApp.main()
