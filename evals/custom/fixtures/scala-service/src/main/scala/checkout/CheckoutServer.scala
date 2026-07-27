// Eval fixture: deliberately uninstrumented Scala HTTP service (see
// evals/fixtures/README.md for the contract). It serves GET /checkout,
// performs one outbound HTTP call to DOWNSTREAM_URL while handling it,
// listens on PORT (default 8080), and uses only obviously synthetic data
// (user@example.test, TEST-0001). JDK classes only: no dependencies.
package checkout

import com.sun.net.httpserver.{HttpExchange, HttpServer}

import java.net.http.{HttpClient, HttpRequest, HttpResponse}
import java.net.{InetSocketAddress, URI}
import java.nio.charset.StandardCharsets
import java.time.Duration
import scala.util.{Failure, Success, Try}

object CheckoutServer {

  def main(args: Array[String]): Unit = {
    val port = sys.env.getOrElse("PORT", "8080").toInt
    val downstreamUrl = sys.env.getOrElse("DOWNSTREAM_URL", "http://localhost:9090/inventory")
    val client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build()

    val server = HttpServer.create(new InetSocketAddress(port), 0)
    server.createContext("/checkout", (exchange: HttpExchange) => handleCheckout(exchange, client, downstreamUrl))
    server.start()
    println(s"scala-service listening on :$port")
  }

  private def handleCheckout(exchange: HttpExchange, client: HttpClient, downstreamUrl: String): Unit = {
    if (exchange.getRequestMethod != "GET") {
      respond(exchange, 405, """{"error":"method not allowed"}""")
      return
    }

    val request = HttpRequest
      .newBuilder(URI.create(downstreamUrl))
      .timeout(Duration.ofSeconds(5))
      .GET()
      .build()

    Try(client.send(request, HttpResponse.BodyHandlers.ofString())) match {
      case Failure(error) =>
        System.err.println(s"inventory lookup failed: $error")
        respond(exchange, 502, """{"error":"inventory unavailable"}""")
      case Success(response) =>
        println("checkout completed order.id=TEST-0001 customer.email=user@example.test")
        val inventory = quote(response.body())
        respond(
          exchange,
          200,
          s"""{"order_id":"TEST-0001","customer_email":"user@example.test","status":"confirmed","inventory":$inventory}"""
        )
    }
  }

  private def respond(exchange: HttpExchange, status: Int, body: String): Unit = {
    val bytes = body.getBytes(StandardCharsets.UTF_8)
    exchange.getResponseHeaders.set("Content-Type", "application/json")
    exchange.sendResponseHeaders(status, bytes.length.toLong)
    val out = exchange.getResponseBody
    try out.write(bytes)
    finally out.close()
  }

  private def quote(raw: String): String =
    "\"" + raw.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n") + "\""
}
