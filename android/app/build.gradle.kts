import java.util.Locale

plugins {
    id("com.android.application")
    id("com.google.devtools.ksp")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

fun secret(name: String): String? = providers.gradleProperty(name).orNull
    ?: providers.environmentVariable(name).orNull

val releaseRequested = gradle.startParameter.taskNames.any { requestedTask ->
    val taskName = requestedTask.substringAfterLast(':').lowercase(Locale.ROOT)
    taskName.contains("release") || taskName == "build" || taskName == "assemble"
}
val signingValues = mapOf(
    "path" to secret("TOKENFLOW_KEYSTORE_PATH"),
    "storePassword" to secret("TOKENFLOW_KEYSTORE_PASSWORD"),
    "alias" to secret("TOKENFLOW_KEY_ALIAS"),
    "keyPassword" to secret("TOKENFLOW_KEY_PASSWORD"),
)

if (releaseRequested) {
    require(signingValues.values.all { !it.isNullOrBlank() }) {
        "Release builds require TOKENFLOW_KEYSTORE_PATH, TOKENFLOW_KEYSTORE_PASSWORD, TOKENFLOW_KEY_ALIAS, and TOKENFLOW_KEY_PASSWORD"
    }
}

android {
    namespace = "com.tokenflow.chat"
    compileSdk = 36
    buildToolsVersion = "36.0.0"

    defaultConfig {
        applicationId = "com.tokenflow.chat"
        minSdk = 26
        targetSdk = 36
        versionCode = 8
        versionName = "2.3.3"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables.useSupportLibrary = true
    }

    signingConfigs {
        if (signingValues.values.all { !it.isNullOrBlank() }) {
            create("release") {
                storeFile = file(signingValues.getValue("path")!!)
                storePassword = signingValues.getValue("storePassword")
                keyAlias = signingValues.getValue("alias")
                keyPassword = signingValues.getValue("keyPassword")
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.findByName("release")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    packaging {
        resources.excludes += setOf("/META-INF/{AL2.0,LGPL2.1}")
    }

    testOptions {
        unitTests.isIncludeAndroidResources = true
    }
}

ksp {
    arg("room.schemaLocation", "$projectDir/schemas")
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2026.06.01")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    implementation("androidx.activity:activity-compose:1.11.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.9.4")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.9.4")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.media3:media3-exoplayer:1.8.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.2")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.9.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("androidx.room:room-runtime:2.8.4")
    implementation("androidx.room:room-ktx:2.8.4")
    implementation("androidx.exifinterface:exifinterface:1.4.2")
    implementation("org.jsoup:jsoup:1.23.1")
    implementation("com.tom-roush:pdfbox-android:2.0.27.0")
    implementation("org.apache.poi:poi-ooxml:5.4.1")
    implementation("org.apache.poi:poi-scratchpad:5.4.1")
    ksp("androidx.room:room-compiler:2.8.4")
    implementation("org.commonmark:commonmark:0.24.0")
    implementation("org.commonmark:commonmark-ext-gfm-tables:0.24.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.10.2")
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")

    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.6.1")
    androidTestImplementation("androidx.room:room-testing:2.8.4")
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    debugImplementation("androidx.compose.ui:ui-tooling")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
