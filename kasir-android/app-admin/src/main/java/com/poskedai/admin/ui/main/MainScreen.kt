package com.poskedai.admin.ui.main

import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Assignment
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.List
import androidx.compose.material.icons.filled.Star
import androidx.compose.material.icons.filled.Store
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.poskedai.admin.ui.dashboard.DashboardScreen
import com.poskedai.admin.ui.product.ProductListScreen
import com.poskedai.admin.ui.product.ImportProductScreen
import com.poskedai.admin.ui.request.RequestProductScreen
import com.poskedai.admin.ui.store.StoreDetailScreen
import com.poskedai.admin.ui.store.StoreListScreen
import com.poskedai.admin.ui.subscription.AdminSubscriptionScreen

sealed class BottomNavItem(val route: String, val title: String, val icon: androidx.compose.ui.graphics.vector.ImageVector) {
    object Dashboard : BottomNavItem("dashboard", "Dashboard", Icons.Default.Home)
    object StoreList : BottomNavItem("stores", "Toko Terdaftar", Icons.Default.Store)
    object ProductList : BottomNavItem("products", "List Produk", Icons.Default.List)
    object RequestProduct : BottomNavItem("requests", "Request Produk", Icons.Default.Assignment)
    object Subscriptions : BottomNavItem("subscriptions", "Paket Pro", Icons.Default.Star)
}

@Composable
fun MainScreen(onLogout: () -> Unit) {
    val navController = rememberNavController()
    val items = listOf(
        BottomNavItem.Dashboard,
        BottomNavItem.StoreList,
        BottomNavItem.ProductList,
        BottomNavItem.RequestProduct,
        BottomNavItem.Subscriptions
    )

    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = navBackStackEntry?.destination
    val showBottomBar = items.any { it.route == currentDestination?.route }

    Scaffold(
        bottomBar = {
            if (showBottomBar) {
                NavigationBar {
                    items.forEach { screen ->
                        NavigationBarItem(
                            icon = { Icon(screen.icon, contentDescription = null) },
                            label = { Text(screen.title) },
                            selected = currentDestination?.hierarchy?.any { it.route == screen.route } == true,
                            onClick = {
                                navController.navigate(screen.route) {
                                    popUpTo(navController.graph.findStartDestination().id) {
                                        saveState = true
                                    }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            }
                        )
                    }
                }
            }
        }
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = BottomNavItem.Dashboard.route,
            modifier = Modifier.padding(innerPadding)
        ) {
            composable(BottomNavItem.Dashboard.route) {
                DashboardScreen()
            }
            composable(BottomNavItem.StoreList.route) {
                StoreListScreen(
                    onNavigateToStoreDetail = { storeId ->
                        navController.navigate("store_detail/$storeId")
                    }
                )
            }
            composable(
                route = "store_detail/{storeId}",
                arguments = listOf(navArgument("storeId") { type = NavType.StringType })
            ) { backStackEntry ->
                val storeId = backStackEntry.arguments?.getString("storeId") ?: ""
                StoreDetailScreen(
                    storeId = storeId,
                    onBackClick = { navController.popBackStack() }
                )
            }
            composable(BottomNavItem.ProductList.route) {
                ProductListScreen(
                    onNavigateToImport = { navController.navigate("import_products") }
                )
            }
            composable("import_products") {
                ImportProductScreen()
            }
            composable(BottomNavItem.RequestProduct.route) {
                RequestProductScreen(
                    onNavigateToEdit = { productJson ->
                        // Pass JSON as argument or store in savedStateHandle
                        navController.currentBackStackEntry?.savedStateHandle?.set("edit_product_json", productJson.toString())
                        navController.navigate("edit_admin_product?isPending=true")
                    }
                )
            }
            composable(
                route = "edit_admin_product?isPending={isPending}",
                arguments = listOf(
                    navArgument("isPending") {
                        type = NavType.BoolType
                        defaultValue = true
                    }
                )
            ) { backStackEntry ->
                val isPending = backStackEntry.arguments?.getBoolean("isPending") ?: true
                val productJsonStr = navController.previousBackStackEntry?.savedStateHandle?.get<String>("edit_product_json") ?: "{}"
                val productJson = com.google.gson.JsonParser.parseString(productJsonStr).asJsonObject

                com.poskedai.admin.ui.product.EditAdminProductScreen(
                    productJson = productJson,
                    isPending = isPending,
                    onBackClick = { navController.popBackStack() },
                    onSuccess = {
                        navController.popBackStack()
                    }
                )
            }
            composable(BottomNavItem.Subscriptions.route) {
                AdminSubscriptionScreen()
            }
        }
    }
}
