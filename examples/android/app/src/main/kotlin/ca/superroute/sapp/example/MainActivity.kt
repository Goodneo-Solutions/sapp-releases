package ca.superroute.sapp.example

import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import ca.superroute.sapp.DEFAULT_BASE_URL
import ca.superroute.sapp.MemoryTokenStore
import ca.superroute.sapp.PartnerClient
import ca.superroute.sapp.ThemeTokens
import ca.superroute.sapp.ui.AccountPrivacy
import ca.superroute.sapp.ui.BoardClaims
import ca.superroute.sapp.ui.Cart
import ca.superroute.sapp.ui.Checkout
import ca.superroute.sapp.ui.DriverDuty
import ca.superroute.sapp.ui.DriverEarnings
import ca.superroute.sapp.ui.DriverPool
import ca.superroute.sapp.ui.DriverRoutes
import ca.superroute.sapp.ui.GroupBuy
import ca.superroute.sapp.ui.GroupBuyLead
import ca.superroute.sapp.ui.GroupBuyLeading
import ca.superroute.sapp.ui.GroupBuyLeadingGroup
import ca.superroute.sapp.ui.GroupBuyList
import ca.superroute.sapp.ui.GroupBuyOrder
import ca.superroute.sapp.ui.GroupBuyOrders
import ca.superroute.sapp.ui.NotificationInbox
import ca.superroute.sapp.ui.NotificationPreferences
import ca.superroute.sapp.ui.OrderDetail
import ca.superroute.sapp.ui.OrderList
import ca.superroute.sapp.ui.ProductDetail
import ca.superroute.sapp.ui.ReturnForm
import ca.superroute.sapp.ui.ReturnList
import ca.superroute.sapp.ui.RouteBoard
import ca.superroute.sapp.ui.RouteBoards
import ca.superroute.sapp.ui.RouteStops
import ca.superroute.sapp.ui.ScanLookup
import ca.superroute.sapp.ui.ShippingOrderForm
import ca.superroute.sapp.ui.SignIn
import ca.superroute.sapp.ui.SrPosition
import ca.superroute.sapp.ui.Stop
import ca.superroute.sapp.ui.StoreDirectory
import ca.superroute.sapp.ui.StorePage
import ca.superroute.sapp.ui.SuperrouteScope
import ca.superroute.sapp.ui.Tracking
import ca.superroute.sapp.ui.WalletSummary
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/**
 * Shows every component of the kit against a client configured on the first screen (an app code and a service
 * provider's client code from the Open Platform console). The kit raises events; this app decides what they open.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { ExampleApp() }
    }
}

/** A screen of the example: a title and what it shows. */
private class Screen(val title: String, val content: @Composable (open: (Screen) -> Unit) -> Unit)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ExampleApp() {
    val context = LocalContext.current
    fun toast(text: String) = Toast.makeText(context, text, Toast.LENGTH_SHORT).show()
    var appCode by rememberSaveable { mutableStateOf("") }
    var clientCode by rememberSaveable { mutableStateOf("") }
    var color by rememberSaveable { mutableStateOf("#1A3D7C") }
    var client by remember { mutableStateOf<PartnerClient?>(null) }
    val stack = remember { mutableStateListOf<Screen>() }
    BackHandler(enabled = stack.isNotEmpty()) { stack.removeAt(stack.lastIndex) }
    val theme = remember(color) { ThemeTokens(JsonObject(mapOf("primary_color" to JsonPrimitive(color)))) }
    val open: (Screen) -> Unit = { stack.add(it) }

    val screens = remember {
        listOf(
            Screen("Sign-in") { SignIn(JsonObject(emptyMap()), "ca.superroute.sapp.example:/oauth", openBrowser = { _, _ -> null }, allowPassword = true, onSignedIn = { toast("Signed in") }) },
            Screen("Notification inbox") { NotificationInbox(onOpen = { toast("Open ${it["deep_link"]}") }) },
            Screen("Wallet summary") { WalletSummary(onTopUp = { min, max, currency -> toast("Top up $min–$max $currency") }) },
            Screen("Order list") { go -> OrderList(onOpen = { type, id -> go(Screen("Order $id") { o -> OrderDetail(type, id, onTrack = { number -> o(Screen("Tracking") { Tracking(number) }) }) }) }) },
            Screen("Tracking") { Tracking() },
            Screen("Notification preferences") { NotificationPreferences() },
            Screen("Account and privacy") { AccountPrivacy(onOpenLink = { toast(it) }) },
            Screen("Return list") { go -> ReturnList(onCreate = { go(Screen("Return form") { ReturnForm(onCreated = { toast("Created") }) }) }) },
            Screen("Shipping order form") { ShippingOrderForm(onCreated = { toast("Order ${it["id"]}") }) },
            Screen("Store directory") { go ->
                StoreDirectory(onOpen = { slug ->
                    go(Screen(slug) { g ->
                        StorePage(slug, onOpenProduct = { s, id -> g(Screen("Product $id") { ProductDetail(s, id, onAdded = { toast("Added") }) }) },
                            onOpenCart = { s -> g(Screen("Cart") { h -> Cart(s, onCheckout = { c -> h(Screen("Checkout") { Checkout(c, onPayment = { _, _ -> toast("Card sheet") }, onPlaced = { toast("Placed") }) }) }) }) })
                    })
                })
            },
            Screen("Group buy list") { go -> GroupBuyList(onOpen = { code -> go(Screen(code) { GroupBuy(code, onPayment = { _, _ -> toast("Card hold sheet") }) }) }) },
            Screen("Group buy orders") { go -> GroupBuyOrders(onOpen = { token -> go(Screen("Order") { GroupBuyOrder(token, onPay = { toast("Pay $it") }) }) }) },
            Screen("Driver routes") { go ->
                DriverRoutes(onOpen = { routeId ->
                    go(Screen("Route $routeId") { g ->
                        RouteStops(routeId, onOpenStop = { r, o -> g(Screen("Stop $o") { Stop(r, o, getPosition = { SrPosition(43.6532, -79.3832) }, onCompleted = { _, s -> toast("Completed $s") }) }) })
                    })
                })
            },
            Screen("Duty and offers") { DriverDuty() },
            Screen("Pool") { DriverPool() },
            Screen("Route boards") { go -> RouteBoards(onOpen = { id -> go(Screen("Board $id") { RouteBoard(id) }) }) },
            Screen("Board claims") { BoardClaims() },
            Screen("Earnings") { DriverEarnings() },
            Screen("Scan lookup") { ScanLookup(onFound = { toast("Found ${it["id"]}") }) },
            Screen("Groups I lead") { go ->
                GroupBuyLeading(onOpen = { id -> go(Screen("Group $id") { GroupBuyLeadingGroup(id, onShare = { _, url -> toast(url) }) }) },
                    onOpenPlan = { code -> go(Screen(code) { GroupBuyLead(code, onShare = { _, url -> toast(url) }) }) })
            },
        )
    }

    Scaffold(topBar = {
        TopAppBar(title = { Text(stack.lastOrNull()?.title ?: "sApp UI kit") }, navigationIcon = {
            if (stack.isNotEmpty()) TextButton(onClick = { stack.removeAt(stack.lastIndex) }) { Text("Back") }
        })
    }) { padding ->
        Column(Modifier.fillMaxSize().padding(padding)) {
            val c = client
            if (c == null) {
                Column(Modifier.padding(16.dp)) {
                    OutlinedTextField(appCode, { appCode = it }, label = { Text("App code") }, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(clientCode, { clientCode = it }, label = { Text("Client code") }, modifier = Modifier.fillMaxWidth())
                    OutlinedTextField(color, { color = it }, label = { Text("Primary colour") }, modifier = Modifier.fillMaxWidth())
                    Button(onClick = {
                        client = PartnerClient(appCode = appCode.trim().ifEmpty { "p_example" }, clientCode = clientCode.trim().ifEmpty { null },
                            tokenStore = MemoryTokenStore(), baseUrl = DEFAULT_BASE_URL, locale = "en")
                    }, modifier = Modifier.padding(top = 16.dp)) { Text("Use this client") }
                }
                return@Column
            }
            SuperrouteScope(c, theme, onSignInRequired = { toast("Sign in again") }) {
                val top = stack.lastOrNull()
                if (top != null) {
                    top.content(open)
                } else {
                    LazyColumn {
                        items(screens) { screen ->
                            Text(screen.title, Modifier.fillMaxWidth().clickable { open(screen) }.padding(16.dp))
                            HorizontalDivider()
                        }
                    }
                }
            }
        }
    }
}
