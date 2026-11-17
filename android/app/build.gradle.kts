plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "io.nullwake.app"
    compileSdk = 35

    defaultConfig {
        applicationId = "io.nullwake.app"
        minSdk = 26
        targetSdk = 35
        versionCode = 5
        versionName = "1.1.0"

        ndk {
            abiFilters += listOf("arm64-v8a")
        }
    }

    val keystorePath = System.getenv("NULLWAKE_KEYSTORE_FILE")
    val hasCiKeystore = !keystorePath.isNullOrBlank() && file(keystorePath).exists()

    signingConfigs {
        if (hasCiKeystore) {
            create("ci") {
                storeFile = file(keystorePath!!)
                storeType = "PKCS12"
                storePassword = System.getenv("NULLWAKE_KEYSTORE_PASS")
                keyAlias = System.getenv("NULLWAKE_KEY_ALIAS")
                keyPassword = System.getenv("NULLWAKE_KEY_PASS")
                enableV1Signing = true
                enableV2Signing = true
                enableV3Signing = true
            }
        }
    }

    val activeSigningConfig = if (hasCiKeystore) {
        signingConfigs.getByName("ci")
    } else {
        signingConfigs.getByName("debug")
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            signingConfig = activeSigningConfig
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
        debug {
            isMinifyEnabled = false
            signingConfig = activeSigningConfig
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.aar"))))
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.ui)
    implementation(libs.androidx.ui.graphics)
    implementation(libs.androidx.ui.tooling.preview)
    implementation(libs.androidx.material3)
    implementation(libs.androidx.material.icons.extended)
    debugImplementation(libs.androidx.ui.tooling)
}
